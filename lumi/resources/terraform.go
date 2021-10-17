package resources

import (
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
	"go.mondoo.io/mondoo/lumi"
	"go.mondoo.io/mondoo/motor/transports"
	"go.mondoo.io/mondoo/motor/transports/terraform"
)

func terraformtransport(t transports.Transport) (*terraform.Transport, error) {
	gt, ok := t.(*terraform.Transport)
	if !ok {
		return nil, errors.New("terraform resource is not supported on this transport")
	}
	return gt, nil
}

func (g *lumiTerraform) id() (string, error) {
	return "terraform", nil
}

func (g *lumiTerraform) GetFiles() ([]interface{}, error) {
	t, err := terraformtransport(g.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	var lumiTerraformFiles []interface{}
	files := t.Parser().Files()
	for path := range files {
		lumiTerraformFile, err := g.Runtime.CreateResource("terraform.file",
			"path", path,
		)
		if err != nil {
			return nil, err
		}
		lumiTerraformFiles = append(lumiTerraformFiles, lumiTerraformFile)
	}

	return lumiTerraformFiles, nil
}

func (g *lumiTerraform) GetBlocks() ([]interface{}, error) {
	t, err := terraformtransport(g.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	files := t.Parser().Files()

	var lumiHclBlocks []interface{}
	for k := range files {
		f := files[k]
		blocks, err := listHclBlocks(g.Runtime, f.Body)
		if err != nil {
			return nil, err
		}
		lumiHclBlocks = append(lumiHclBlocks, blocks...)
	}
	return lumiHclBlocks, nil
}

func (g *lumiTerraform) filterBlockByType(filterType string) ([]interface{}, error) {
	t, err := terraformtransport(g.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	files := t.Parser().Files()

	var lumiHclBlocks []interface{}
	for k := range files {
		f := files[k]
		blocks, err := listHclBlocks(g.Runtime, f.Body)
		if err != nil {
			return nil, err
		}

		for i := range blocks {
			b := blocks[i].(TerraformBlock)
			blockType, err := b.Type()
			if err != nil {
				return nil, err
			}
			if blockType == filterType {
				lumiHclBlocks = append(lumiHclBlocks, b)
			}
		}
	}
	return lumiHclBlocks, nil
}

func (g *lumiTerraform) GetProviders() ([]interface{}, error) {
	return g.filterBlockByType("provider")
}

func (g *lumiTerraform) GetDatasources() ([]interface{}, error) {
	return g.filterBlockByType("data")
}

func (g *lumiTerraform) GetResources() ([]interface{}, error) {
	return g.filterBlockByType("resource")
}

func (g *lumiTerraform) GetVariables() ([]interface{}, error) {
	return g.filterBlockByType("variable")
}

func (g *lumiTerraform) GetOutputs() ([]interface{}, error) {
	return g.filterBlockByType("output")
}

func newLumiHclBlock(runtime *lumi.Runtime, block *hcl.Block) (lumi.ResourceType, error) {
	start, end, err := newFilePosRange(runtime, block.TypeRange)
	if err != nil {
		return nil, err
	}

	r, err := runtime.CreateResource("terraform.block",
		"type", block.Type,
		"labels", sliceInterface(block.Labels),
		"start", start,
		"end", end,
	)

	if err == nil {
		r.LumiResource().Cache.Store("_hclblock", &lumi.CacheEntry{Data: block})
	}

	return r, err
}

func (g *lumiTerraformBlock) id() (string, error) {
	// NOTE: a hcl block is identified by its filename and position
	fp, err := g.Start()
	if err != nil {
		return "", err
	}
	file, _ := fp.Path()
	line, _ := fp.Line()
	column, _ := fp.Column()

	return "terraform.block/" + file + "/" + strconv.FormatInt(line, 10) + "/" + strconv.FormatInt(column, 10), nil
}

func (g *lumiTerraformBlock) GetNameLabel() (interface{}, error) {
	labels, err := g.Labels()
	if err != nil {
		return nil, err
	}

	// labels are string
	if len(labels) == 0 {
		return "", nil
	}

	return labels[0].(string), nil
}

func (g *lumiTerraformBlock) GetArguments() (map[string]interface{}, error) {
	ce, ok := g.LumiResource().Cache.Load("_hclblock")
	if !ok {
		return nil, nil
	}

	hclBlock := ce.Data.(*hcl.Block)

	// do not handle diag information here, it also throws errors for blocks nearby
	attributes, _ := hclBlock.Body.JustAttributes()

	dict := map[string]interface{}{}
	for k := range attributes {
		dict[k] = getCtyValue(attributes[k].Expr, nil)
	}

	return dict, nil
}

func getCtyValue(expr hcl.Expression, ctx *hcl.EvalContext) interface{} {
	switch t := expr.(type) {
	case *hclsyntax.TupleConsExpr:
		results := []interface{}{}
		for _, expr := range t.Exprs {
			res := getCtyValue(expr, ctx)
			switch v := res.(type) {
			case []interface{}:
				results = append(results, v...)
			default:
				results = append(results, v)
			}
		}
		return results
	case *hclsyntax.ScopeTraversalExpr:
		traversal := t.Variables()
		res := []string{}
		for i := range traversal {
			tr := traversal[i]
			for j := range tr {
				switch v := tr[j].(type) {
				case hcl.TraverseRoot:
					res = append(res, v.Name)
				case hcl.TraverseAttr:
					res = append(res, v.Name)
				}
			}
		}
		// TODO: are we sure we want to do this?
		return strings.Join(res, ".")
	case *hclsyntax.FunctionCallExpr, *hclsyntax.ConditionalExpr:
		results := []interface{}{}
		subVal, err := t.Value(ctx)
		if err == nil && subVal.Type() == cty.String {
			results = append(results, subVal.AsString())
		}
		return results
	case *hclsyntax.LiteralValueExpr:
		switch t.Val.Type() {
		case cty.String:
			return t.Val.AsString()
		case cty.Bool:
			return t.Val.True()
		default:
			log.Warn().Msgf("unknown type %T", t)
			return nil
		}
	case *hclsyntax.TemplateExpr:
		// walk the parts of the expression to ensure that it has a literal value

		if len(t.Parts) == 1 {
			return getCtyValue(t.Parts[0], ctx)
		}

		results := []interface{}{}
		for _, p := range t.Parts {
			res := getCtyValue(p, ctx)
			switch v := res.(type) {
			case []interface{}:
				results = append(results, v...)
			default:
				results = append(results, v)
			}
		}
		return results
	default:
		log.Warn().Msgf("unknown type %T", t)
		return nil
	}
	return nil
}

func (g *lumiTerraformBlock) GetBlocks() ([]interface{}, error) {
	ce, ok := g.LumiResource().Cache.Load("_hclblock")
	if !ok {
		return nil, nil
	}

	hclBlock := ce.Data.(*hcl.Block)
	return listHclBlocks(g.Runtime, hclBlock.Body)
}

func listHclBlocks(runtime *lumi.Runtime, rawBody interface{}) ([]interface{}, error) {
	var lumiHclBlocks []interface{}
	switch body := rawBody.(type) {
	case *hclsyntax.Body:
		for i := range body.Blocks {
			lumiBlock, err := newLumiHclBlock(runtime, body.Blocks[i].AsHCLBlock())
			if err != nil {
				return nil, err
			}
			lumiHclBlocks = append(lumiHclBlocks, lumiBlock)
		}
	case hcl.Body:
		content, _, _ := body.PartialContent(terraform.TerraformSchema_0_12)
		for i := range content.Blocks {
			lumiBlock, err := newLumiHclBlock(runtime, content.Blocks[i])
			if err != nil {
				return nil, err
			}
			lumiHclBlocks = append(lumiHclBlocks, lumiBlock)
		}
	default:
		return nil, errors.New("unsupported hcl block type")
	}

	return lumiHclBlocks, nil
}

func newFilePosRange(runtime *lumi.Runtime, r hcl.Range) (lumi.ResourceType, lumi.ResourceType, error) {
	start, err := runtime.CreateResource("terraform.fileposition",
		"path", r.Filename,
		"line", int64(r.Start.Line),
		"column", int64(r.Start.Column),
		"byte", int64(r.Start.Byte),
	)
	if err != nil {
		return nil, nil, err
	}

	end, err := runtime.CreateResource("terraform.fileposition",
		"path", r.Filename,
		"line", int64(r.Start.Line),
		"column", int64(r.Start.Column),
		"byte", int64(r.Start.Byte),
	)
	if err != nil {
		return nil, nil, err
	}

	return start, end, nil
}

func (p *lumiTerraformFileposition) id() (string, error) {
	path, _ := p.Path()
	line, _ := p.Line()
	column, _ := p.Column()
	return "file.position/" + path + "/" + strconv.FormatInt(line, 10) + "/" + strconv.FormatInt(column, 10), nil
}

func (g *lumiTerraformFile) id() (string, error) {
	p, err := g.Path()
	if err != nil {
		return "", err
	}
	return "terraform.file/" + p, nil
}

func (g *lumiTerraformFile) GetBlocks() ([]interface{}, error) {
	t, err := terraformtransport(g.Runtime.Motor.Transport)
	if err != nil {
		return nil, err
	}

	p, err := g.Path()
	if err != nil {
		return nil, err
	}

	files := t.Parser().Files()
	file := files[p]
	return listHclBlocks(g.Runtime, file.Body)
}