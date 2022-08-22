package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mondoo.io/mondoo/llx"
	"go.mondoo.io/mondoo/motor"
	"go.mondoo.io/mondoo/motor/providers"
	"go.mondoo.io/mondoo/motor/providers/terraform"
	"go.mondoo.io/mondoo/resources/packs/core"
	"go.mondoo.io/mondoo/resources/packs/testutils"
)

func terraformTestQuery(t *testing.T, query string) []*llx.RawResult {
	p, err := terraform.New(&providers.Config{
		Backend: providers.ProviderType_TERRAFORM,
		Options: map[string]string{
			"path": "./terraform/testdata",
		},
	})
	require.NoError(t, err)

	m, err := motor.New(p)
	require.NoError(t, err)

	x := testutils.InitTester(m, core.Registry)
	return x.TestQuery(t, query)
}

func TestResource_Terraform(t *testing.T) {
	t.Run("terraform providers", func(t *testing.T) {
		res := terraformTestQuery(t, "terraform.providers[0].type")
		require.NotEmpty(t, res)
		assert.Empty(t, res[0].Result().Error)
		assert.Equal(t, string("provider"), res[0].Data.Value)
	})

	t.Run("terraform ignore commented out resources", func(t *testing.T) {
		res := terraformTestQuery(t, "terraform.providers.length")
		require.NotEmpty(t, res)
		assert.Empty(t, res[0].Result().Error)
		assert.Equal(t, int64(2), res[0].Data.Value)
	})

	t.Run("terraform nested blocks", func(t *testing.T) {
		res := terraformTestQuery(t, "terraform.blocks.where( type == \"resource\" && labels.contains(\"aws_instance\"))[0].type")
		require.NotEmpty(t, res)
		assert.Empty(t, res[0].Result().Error)
		assert.Equal(t, string("resource"), res[0].Data.Value)
	})

	t.Run("terraform jsonencode blocks", func(t *testing.T) {
		res := terraformTestQuery(t, "terraform.resources.where( nameLabel == 'aws_iam_policy' && labels[1] == 'policy' )[0].arguments['policy'][0]['Version']")
		require.NotEmpty(t, res)
		assert.Empty(t, res[0].Result().Error)
		assert.Equal(t, string("2012-10-17"), res[0].Data.Value)
	})

	t.Run("terraform providers", func(t *testing.T) {
		res := terraformTestQuery(t, "terraform.resources.where( nameLabel  == 'google_compute_instance')[0].arguments['metadata']")
		require.NotEmpty(t, res)
		assert.Empty(t, res[0].Result().Error)
		assert.Equal(t, map[string]interface{}{"enable-oslogin": true}, res[0].Data.Value)
	})

	t.Run("terraform settings", func(t *testing.T) {
		res := terraformTestQuery(t, "terraform.settings.requiredProviders['aws']['version']")
		require.NotEmpty(t, res)
		assert.Empty(t, res[0].Result().Error)
		assert.Equal(t, "~> 3.74", res[0].Data.Value)
	})
}