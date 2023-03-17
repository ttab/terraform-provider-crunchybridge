/*
Copyright 2022 Crunchy Data Solutions, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/CrunchyData/terraform-provider-crunchybridge/internal/bridgeapi"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	idConfigName             = "application_id"
	secretConfigName         = "application_secret"
	apiKeyConfigName         = "api_key"
	urlConfigName            = "bridgeapi_url"
	immediateLoginConfigName = "immediate_login"
)

func init() {
	// Set descriptions to support markdown syntax, this will be used in document generation
	// and the language server.
	schema.DescriptionKind = schema.StringMarkdown

	// Customize the content of descriptions when output. For example you can add defaults on
	// to the exported descriptions if present.
	// schema.SchemaDescriptionBuilder = func(s *schema.Schema) string {
	// 	desc := s.Description
	// 	if s.Default != nil {
	// 		desc += fmt.Sprintf(" Defaults to `%v`.", s.Default)
	// 	}
	// 	return strings.TrimSpace(desc)
	// }
}

func New(version string) func() *schema.Provider {
	return func() *schema.Provider {
		p := &schema.Provider{
			DataSourcesMap: map[string]*schema.Resource{
				"crunchybridge_account":       dataSourceAccount(),
				"crunchybridge_cloudprovider": dataSourceCloudProvider(),
				"crunchybridge_cluster":       dataSourceCluster(),
				"crunchybridge_clusterids":    dataSourceClusterIDs(),
				"crunchybridge_clusterroles":  dataSourceRoles(),
				"crunchybridge_clusterstatus": dataSourceStatus(),
			},
			ResourcesMap: map[string]*schema.Resource{
				"crunchybridge_cluster": resourceCluster(),
			},
			Schema: map[string]*schema.Schema{
				apiKeyConfigName: {
					Type:        schema.TypeString,
					Description: "The application id component of the Crunchy Bridge API key.",
					DefaultFunc: schema.EnvDefaultFunc("API_KEY", ""),
					Optional:    true,
				},
				idConfigName: {
					Type:        schema.TypeString,
					Description: "The application id component of the Crunchy Bridge API key.",
					DefaultFunc: schema.EnvDefaultFunc("APPLICATION_ID", ""),
					Optional:    true,
				},
				secretConfigName: {
					Type:        schema.TypeString,
					Description: "The application secret component of the Crunchy Bridge API key.",
					DefaultFunc: schema.EnvDefaultFunc("APPLICATION_SECRET", ""),
					Optional:    true,
				},
				immediateLoginConfigName: {
					Type: schema.TypeBool,
					Description: fmt.Sprintf("When true, %q and %q will be validated when the provider is configured.",
						idConfigName, secretConfigName),
					Optional: true,
				},
				urlConfigName: {
					Type:        schema.TypeString,
					Description: "The API URL for the Crunchy Bridge platform API. Most users should not need to change this value.",
					DefaultFunc: schema.EnvDefaultFunc("BRIDGE_API_URL", "https://api.crunchybridge.com"),
					Required:    true,
				},
			},
		}

		p.ConfigureContextFunc = configure(version, p)

		return p
	}
}

func configure(version string, p *schema.Provider) func(context.Context, *schema.ResourceData) (interface{}, diag.Diagnostics) {
	return func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		// Provider.UserAgent provides a UserAgent string with the
		// passed parameters, Terraform version, SDK version, and other
		// bits:
		userAgent := p.UserAgent("terraform-provider-crunchybridge", version)

		id := d.Get(idConfigName).(string)
		secret := d.Get(secretConfigName).(string)
		apiKey := d.Get(apiKeyConfigName).(string)
		immediateLogin := d.Get(immediateLoginConfigName).(bool)

		var token bridgeapi.TokenSource

		switch {
		case id != "" && secret != "":
			token = bridgeapi.NewLegacyAuth(id, secret)
		case apiKey != "":
			token = bridgeapi.APIKeyAuth(apiKey)
		default:
			return nil, diag.Errorf(
				"either supply %q or %q and %q for authentication",
				apiKeyConfigName, idConfigName, secretConfigName)
		}

		apiUrl, err := url.Parse(d.Get(urlConfigName).(string))
		if err != nil {
			return nil, diag.Errorf(
				"invalid %q: %v", urlConfigName, err)
		}

		options := []bridgeapi.ClientOption{
			bridgeapi.WithUserAgent(userAgent),
		}

		if immediateLogin {
			options = append(options, bridgeapi.WithImmediateLogin())
		}

		c, err := bridgeapi.NewClient(apiUrl, token, options...)
		if err != nil {
			return nil, diag.FromErr(err)
		}

		return c, nil
	}
}
