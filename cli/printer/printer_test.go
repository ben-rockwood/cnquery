package printer

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mondoo.com/cnquery"
	"go.mondoo.com/cnquery/llx"
	"go.mondoo.com/cnquery/logger"
	"go.mondoo.com/cnquery/mql"
	"go.mondoo.com/cnquery/mqlc"
	"go.mondoo.com/cnquery/providers"
	"go.mondoo.com/cnquery/providers/mock"
	"go.mondoo.com/cnquery/resources"
	"go.mondoo.com/cnquery/sortx"
)

var features cnquery.Features

func init() {
	logger.InitTestEnv()
	features = getEnvFeatures()
}

func getEnvFeatures() cnquery.Features {
	env := os.Getenv("FEATURES")
	if env == "" {
		return cnquery.Features{byte(cnquery.PiperCode)}
	}

	arr := strings.Split(env, ",")
	var fts cnquery.Features
	for i := range arr {
		v, ok := cnquery.FeaturesValue[arr[i]]
		if ok {
			fmt.Println("--> activate feature: " + arr[i])
			fts = append(features, byte(v))
		} else {
			panic("cannot find requested feature: " + arr[i])
		}
	}
	return fts
}

func executionContext() (*resources.Schema, llx.Runtime) {
	m, err := mock.NewFromTomlFile("../../mql/testdata/arch.toml")
	if err != nil {
		panic(err.Error())
	}
	if err = m.LoadSchemas(providers.Coordinator.LoadSchema); err != nil {
		panic(err.Error())
	}

	return m.Schema(), m
}

func testQuery(t *testing.T, query string) (*llx.CodeBundle, map[string]*llx.RawResult) {
	schema, runtime := executionContext()
	codeBundle, err := mqlc.Compile(query, nil, mqlc.NewConfig(schema, features))
	require.NoError(t, err)

	results, err := mql.ExecuteCode(schema, runtime, codeBundle, nil, features)
	require.NoError(t, err)

	return codeBundle, results
}

type simpleTest struct {
	code     string
	expected string
	results  []string
}

func runSimpleTests(t *testing.T, tests []simpleTest) {
	for i := range tests {
		cur := tests[i]
		t.Run(cur.code, func(t *testing.T) {
			bundle, results := testQuery(t, cur.code)

			s := DefaultPrinter.CodeBundle(bundle)
			if cur.expected != "" {
				assert.Equal(t, cur.expected, s)
			}

			length := len(results)

			assert.Equal(t, length, len(cur.results), "make sure the right number of results are returned")

			keys := sortx.Keys(results)
			for idx, id := range keys {
				result := results[id]
				s = DefaultPrinter.Result(result, bundle)
				assert.Equal(t, cur.results[idx], s)
			}
		})
	}
}

type assessmentTest struct {
	code   string
	result string
}

func runAssessmentTests(t *testing.T, tests []assessmentTest) {
	for i := range tests {
		cur := tests[i]
		t.Run(cur.code, func(t *testing.T) {
			bundle, resultsMap := testQuery(t, cur.code)

			raw := DefaultPrinter.Results(bundle, resultsMap)
			assert.Equal(t, cur.result, raw)
		})
	}
}

func TestPrinter(t *testing.T) {
	runSimpleTests(t, []simpleTest{
		{
			"if ( mondoo.version != null ) { mondoo.build }",
			"", // ignore
			[]string{
				"mondoo.version: \"unstable\"",
				"if: {\n" +
					"  mondoo.build: \"development\"\n" +
					"}",
			},
		},
		{
			"file('zzz') { content }",
			"",
			[]string{
				"error: Query encountered errors:\n" +
					"1 error occurred:\n" +
					"\t* file not found: 'zzz' does not exist\n" +
					"file: {\n  content: error: file not found: 'zzz' does not exist\n}",
			},
		},
		{
			"[]",
			"", // ignore
			[]string{
				"[]",
			},
		},
		{
			"{}",
			"", // ignore
			[]string{
				"{}",
			},
		},
		{
			"['1-2'] { _.split('-') }",
			"", // ignore
			[]string{
				"[\n" +
					"  0: {\n" +
					"    split: [\n" +
					"      0: \"1\"\n" +
					"      1: \"2\"\n" +
					"    ]\n" +
					"  }\n" +
					"]",
			},
		},
		{
			"mondoo { version }",
			"-> block 1\n   entrypoints: [<1,2>]\n   1: mondoo \n   2: {} bind: <1,1> type:block (=> <2,0>)\n-> block 2\n   entrypoints: [<2,2>]\n   1: mondoo id = context\n   2: version bind: <2,1> type:string\n",
			[]string{
				"mondoo: {\n  version: \"unstable\"\n}",
			},
		},
		{
			"mondoo { _.version }",
			"-> block 1\n   entrypoints: [<1,2>]\n   1: mondoo \n   2: {} bind: <1,1> type:block (=> <2,0>)\n-> block 2\n   entrypoints: [<2,2>]\n   1: mondoo id = context\n   2: version bind: <2,1> type:string\n",
			[]string{
				"mondoo: {\n  version: \"unstable\"\n}",
			},
		},
		{
			"[1].where( _ > 0 )",
			"-> block 1\n   entrypoints: [<1,2>]\n   1: [\n     0: 1\n   ]\n   2: where bind: <1,1> type:[]int (ref<1,1>, => <2,0>)\n-> block 2\n   entrypoints: [<2,2>]\n   1: _\n   2: >\005 bind: <2,1> type:bool (0)\n",
			[]string{
				"where: [\n  0: 1\n]",
			},
		},
		{
			"a = 3\n if(true) {\n a == 3 \n}",
			"-> block 1\n   entrypoints: [<1,2>]\n   1: 3\n   2: if bind: <0,0> type:block (true, => <2,0>, [\n     0: ref<1,1>\n   ])\n-> block 2\n   entrypoints: [<2,2>]\n   1: ref<1,1>\n   2: ==\x05 bind: <2,1> type:bool (3)\n",
			[]string{"if: {\n  a == 3: true\n}"},
		},
		{
			"mondoo",
			"", // ignore
			[]string{
				"mondoo: mondoo version=\"unstable\"",
			},
		},
		{
			"users",
			"", // ignore
			[]string{
				"users.list: [\n" +
					"  0: user name=\"root\" uid=0 gid=0\n" +
					"  1: user name=\"chris\" uid=1000 gid=1001\n" +
					"  2: user name=\"christopher\" uid=1000 gid=1001\n" +
					"  3: user name=\"chris\" uid=1002 gid=1003\n" +
					"  4: user name=\"bin\" uid=1 gid=1\n" +
					"]",
			},
		},
	})
}

func TestPrinter_Assessment(t *testing.T) {
	runAssessmentTests(t, []assessmentTest{
		{
			// mixed use: assertion and erroneous data field
			"mondoo.build == 1; user(name: 'notthere').authorizedkeys.file",
			strings.Join([]string{
				"[failed] mondoo.build == 1; user(name: 'notthere').authorizedkeys.file",
				"  [failed] mondoo.build == 1",
				"    expected: == 1",
				"    actual:   \"development\"",
				"  [failed] user.authorizedkeys.file",
				"    error: failed to create resource 'user': user 'notthere' does not exist",
				"",
			}, "\n"),
		},
		{
			// mixed use: assertion and working data field
			"mondoo.build == 1; sshd.config",
			strings.Join([]string{
				"[failed] mondoo.build == 1; sshd.config",
				"  [failed] mondoo.build == 1",
				"    expected: == 1",
				"    actual:   \"development\"",
				"  [ok] value: sshd.config id = /etc/ssh/sshd_config",
				"",
			}, "\n"),
		},
		{
			"[1,2,3].\n" +
				"# @msg Found ${length} numbers\n" +
				"none( _ > 1 )",
			strings.Join([]string{
				"[failed] Found 2 numbers",
				"",
			}, "\n"),
		},
		{
			"# @msg Expected ${$expected.length} users but got ${length}\n" +
				"users.none( uid == 0 )",
			strings.Join([]string{
				"[failed] Expected 5 users but got 1",
				"",
			}, "\n"),
		},
		{
			"mondoo.build == 1",
			strings.Join([]string{
				"[failed] mondoo.build == 1",
				"  expected: == 1",
				"  actual:   \"development\"",
				"",
			}, "\n"),
		},
		{
			"sshd.config { params['test'] }",
			strings.Join([]string{
				"sshd.config: {",
				"  params[test]: null",
				"}",
			}, "\n"),
		},
		{
			"mondoo.build == 1;mondoo.version =='unstable';",
			strings.Join([]string{
				"[failed] mondoo.build == 1;mondoo.version =='unstable';",
				"  [failed] mondoo.build == 1",
				"    expected: == 1",
				"    actual:   \"development\"",
				"  [ok] value: \"unstable\"",
				"",
			}, "\n"),
		},
		{
			"if(true) {\n" +
				"  # @msg Expected ${$expected.length} users but got ${length}\n" +
				"  users.none( uid == 0 )\n" +
				"}",
			strings.Join([]string{
				"if: {",
				"  [failed] Expected 5 users but got 1",
				"  users.where.list: [",
				"  0: user name=\"root\" uid=0 gid=0",
				"  ]",
				"}",
			}, "\n"),
		},
		{
			"users.list.duplicates(gid).none()\n",
			strings.Join([]string{
				"[failed] [].none()",
				"  actual:   [",
				"    0: user {",
				"      name: \"christopher\"",
				"      gid: 1001",
				"      uid: 1000",
				"    }",
				"    1: user {",
				"      name: \"chris\"",
				"      gid: 1001",
				"      uid: 1000",
				"    }",
				"  ]",
				"",
			}, "\n"),
		},
		{
			"users.all( uid < 1000 )\n",
			strings.Join([]string{
				"[failed] users.all()",
				"  actual:   [",
				"    0: user {",
				"      uid: 1000",
				"      gid: 1001",
				"      name: \"chris\"",
				"    }",
				"    1: user {",
				"      uid: 1000",
				"      gid: 1001",
				"      name: \"christopher\"",
				"    }",
				"    2: user {",
				"      uid: 1002",
				"      gid: 1003",
				"      name: \"chris\"",
				"    }",
				"  ]",
				"",
			}, "\n"),
		},
		{
			"users.all( 1000 > uid )\n",
			strings.Join([]string{
				"[failed] users.all()",
				"  actual:   [",
				"    0: user {",
				"      name: \"chris\"",
				"      uid: 1000",
				"      gid: 1001",
				"    }",
				"    1: user {",
				"      name: \"christopher\"",
				"      uid: 1000",
				"      gid: 1001",
				"    }",
				"    2: user {",
				"      name: \"chris\"",
				"      uid: 1002",
				"      gid: 1003",
				"    }",
				"  ]",
				"",
			}, "\n"),
		},
		{
			"users.all( uid == 0 && enabled == true )\n",
			strings.Join([]string{
				"[failed] users.all()",
				"  actual:   [",
				"    0: user {",
				"      gid: 0",
				"      name: \"root\"",
				"      uid: 0",
				"      enabled: false",
				"    }",
				"    1: user {",
				"      gid: 1001",
				"      name: \"chris\"",
				"      uid: 1000",
				"      enabled: false",
				"    }",
				"    2: user {",
				"      gid: 1001",
				"      name: \"christopher\"",
				"      uid: 1000",
				"      enabled: false",
				"    }",
				"    3: user {",
				"      gid: 1003",
				"      name: \"chris\"",
				"      uid: 1002",
				"      enabled: false",
				"    }",
				"    4: user {",
				"      gid: 1",
				"      name: \"bin\"",
				"      uid: 1",
				"      enabled: false",
				"    }",
				"  ]",
				"",
			}, "\n"),
		},
		{
			"users.none( '/root' == home ); users.all( name != 'root' )\n",
			strings.Join([]string{
				"[failed] users.none( '/root' == home ); users.all( name != 'root' )",
				"",
				"  [failed] users.none()",
				"    actual:   [",
				"      0: user {",
				"        gid: 0",
				"        home: \"/root\"",
				"        name: \"root\"",
				"        uid: 0",
				"      }",
				"    ]",
				"  [failed] users.all()",
				"    actual:   [",
				"      0: user {",
				"        name: \"root\"",
				"        uid: 0",
				"        gid: 0",
				"      }",
				"    ]",
				"",
			}, "\n"),
		},
		{
			"users.all(groups.none(gid==0))\n",
			strings.Join([]string{
				"[failed] users.all()",
				"  actual:   [",
				"    0: user {",
				"      uid: 0",
				"      groups.list: [",
				"        0: user group id = group/0/root",
				"        1: user group id = group/1001/chris",
				"        2: user group id = group/90/network",
				"        3: user group id = group/998/wheel",
				"        4: user group id = group/5/tty",
				"        5: user group id = group/2/daemon",
				"      ]",
				"      gid: 0",
				"      groups: groups id = groups",
				"      name: \"root\"",
				"    }",
				"    1: user {",
				"      uid: 1000",
				"      groups.list: [",
				"        0: user group id = group/0/root",
				"        1: user group id = group/1001/chris",
				"        2: user group id = group/90/network",
				"        3: user group id = group/998/wheel",
				"        4: user group id = group/5/tty",
				"        5: user group id = group/2/daemon",
				"      ]",
				"      gid: 1001",
				"      groups: groups id = groups",
				"      name: \"chris\"",
				"    }",
				"    2: user {",
				"      uid: 1000",
				"      groups.list: [",
				"        0: user group id = group/0/root",
				"        1: user group id = group/1001/chris",
				"        2: user group id = group/90/network",
				"        3: user group id = group/998/wheel",
				"        4: user group id = group/5/tty",
				"        5: user group id = group/2/daemon",
				"      ]",
				"      gid: 1001",
				"      groups: groups id = groups",
				"      name: \"christopher\"",
				"    }",
				"    3: user {",
				"      uid: 1002",
				"      groups.list: [",
				"        0: user group id = group/0/root",
				"        1: user group id = group/1001/chris",
				"        2: user group id = group/90/network",
				"        3: user group id = group/998/wheel",
				"        4: user group id = group/5/tty",
				"        5: user group id = group/2/daemon",
				"      ]",
				"      gid: 1003",
				"      groups: groups id = groups",
				"      name: \"chris\"",
				"    }",
				"    4: user {",
				"      uid: 1",
				"      groups.list: [",
				"        0: user group id = group/0/root",
				"        1: user group id = group/1001/chris",
				"        2: user group id = group/90/network",
				"        3: user group id = group/998/wheel",
				"        4: user group id = group/5/tty",
				"        5: user group id = group/2/daemon",
				"      ]",
				"      gid: 1",
				"      groups: groups id = groups",
				"      name: \"bin\"",
				"    }",
				"  ]",
				"",
			}, "\n"),
		},
		{
			"users.all(groups.all(name == 'root'))\n",
			strings.Join([]string{
				"[failed] users.all()",
				"  actual:   [",
				"    0: user {",
				"      uid: 0",
				"      groups.list: [",
				"        0: user group id = group/0/root",
				"        1: user group id = group/1001/chris",
				"        2: user group id = group/90/network",
				"        3: user group id = group/998/wheel",
				"        4: user group id = group/5/tty",
				"        5: user group id = group/2/daemon",
				"      ]",
				"      groups: groups id = groups",
				"      gid: 0",
				"      name: \"root\"",
				"    }",
				"    1: user {",
				"      uid: 1000",
				"      groups.list: [",
				"        0: user group id = group/0/root",
				"        1: user group id = group/1001/chris",
				"        2: user group id = group/90/network",
				"        3: user group id = group/998/wheel",
				"        4: user group id = group/5/tty",
				"        5: user group id = group/2/daemon",
				"      ]",
				"      groups: groups id = groups",
				"      gid: 1001",
				"      name: \"chris\"",
				"    }",
				"    2: user {",
				"      uid: 1000",
				"      groups.list: [",
				"        0: user group id = group/0/root",
				"        1: user group id = group/1001/chris",
				"        2: user group id = group/90/network",
				"        3: user group id = group/998/wheel",
				"        4: user group id = group/5/tty",
				"        5: user group id = group/2/daemon",
				"      ]",
				"      groups: groups id = groups",
				"      gid: 1001",
				"      name: \"christopher\"",
				"    }",
				"    3: user {",
				"      uid: 1002",
				"      groups.list: [",
				"        0: user group id = group/0/root",
				"        1: user group id = group/1001/chris",
				"        2: user group id = group/90/network",
				"        3: user group id = group/998/wheel",
				"        4: user group id = group/5/tty",
				"        5: user group id = group/2/daemon",
				"      ]",
				"      groups: groups id = groups",
				"      gid: 1003",
				"      name: \"chris\"",
				"    }",
				"    4: user {",
				"      uid: 1",
				"      groups.list: [",
				"        0: user group id = group/0/root",
				"        1: user group id = group/1001/chris",
				"        2: user group id = group/90/network",
				"        3: user group id = group/998/wheel",
				"        4: user group id = group/5/tty",
				"        5: user group id = group/2/daemon",
				"      ]",
				"      groups: groups id = groups",
				"      gid: 1",
				"      name: \"bin\"",
				"    }",
				"  ]",
				"",
			}, "\n"),
		},
		{
			"users.all(sshkeys.length > 2)\n",
			strings.Join([]string{
				"[failed] users.all()",
				"  actual:   [",
				"    0: user {",
				"      name: \"root\"",
				"      gid: 0",
				"      sshkeys: []",
				"      sshkeys.length: 0",
				"      uid: 0",
				"    }",
				"    1: user {",
				"      name: \"chris\"",
				"      gid: 1001",
				"      sshkeys: []",
				"      sshkeys.length: 0",
				"      uid: 1000",
				"    }",
				"    2: user {",
				"      name: \"christopher\"",
				"      gid: 1001",
				"      sshkeys: []",
				"      sshkeys.length: 0",
				"      uid: 1000",
				"    }",
				"    3: user {",
				"      name: \"chris\"",
				"      gid: 1003",
				"      sshkeys: []",
				"      sshkeys.length: 0",
				"      uid: 1002",
				"    }",
				"    4: user {",
				"      name: \"bin\"",
				"      gid: 1",
				"      sshkeys: []",
				"      sshkeys.length: 0",
				"      uid: 1",
				"    }",
				"  ]",
				"",
			}, "\n"),
		},
	})
}

func TestPrinter_Blocks(t *testing.T) {
	runSimpleTests(t, []simpleTest{
		{
			"['a', 'b'] { x=_ \n x }",
			"", // ignore
			[]string{
				strings.Join([]string{
					"[",
					"  0: {",
					"    x: \"a\"",
					"  }",
					"  1: {",
					"    x: \"b\"",
					"  }",
					"]",
				}, "\n"),
			},
		},
		{
			"['a', 'b'] { x=_ \n x == 'a' }",
			"", // ignore
			[]string{
				strings.Join([]string{
					"[",
					"  0: {",
					"    x == \"a\": true",
					"  }",
					"  1: {",
					"    x == \"a\": false",
					"  }",
					"]",
				}, "\n"),
			},
		},
	})
}

func TestPrinter_Buggy(t *testing.T) {
	runSimpleTests(t, []simpleTest{
		{
			"mondoo",
			"", // ignore
			[]string{
				"mondoo: mondoo version=\"unstable\"",
			},
		},
	})
}
