// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package command

import (
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/hashicorp/vault/command/agent/config"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

var (
	_ cli.Command             = (*AgentGenerateConfigCommand)(nil)
	_ cli.CommandAutocomplete = (*AgentGenerateConfigCommand)(nil)
)

type AgentGenerateConfigCommand struct {
	*BaseCommand

	flagType  string
	flagPaths []string
	flagExec  string
}

func (c *AgentGenerateConfigCommand) Synopsis() string {
	return "Generate a Vault Agent configuration file."
}

func (c *AgentGenerateConfigCommand) Help() string {
	helpText := `
Usage: vault agent generate-config [options]
` + c.Flags().Help()
	return strings.TrimSpace(helpText)
}

func (c *AgentGenerateConfigCommand) Flags() *FlagSets {
	set := NewFlagSets(c.UI)

	// Common Options
	f := set.NewFlagSet("Command Options")

	f.StringVar(&StringVar{
		Name:    "type",
		Target:  &c.flagType,
		Default: "env-template",
		Usage:   "The type of configuration file to generate; currently, only 'env-template' is supported.",
		Completion: complete.PredictSet(
			"env-template",
		),
	})

	f.StringSliceVar(&StringSliceVar{
		Name:       "path",
		Target:     &c.flagPaths,
		Usage:      "Path to a KV v1/v2 secret (e.g. secret/data/foo, secret/prefix/*).",
		Completion: c.PredictVaultFolders(),
	})

	f.StringVar(&StringVar{
		Name:    "exec",
		Target:  &c.flagExec,
		Default: "env",
		Usage:   "The command to execute for in env-template mode.",
	})

	return set
}

func (c *AgentGenerateConfigCommand) AutocompleteArgs() complete.Predictor {
	return complete.PredictNothing
}

func (c *AgentGenerateConfigCommand) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *AgentGenerateConfigCommand) Run(args []string) int {
	flags := c.Flags()

	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	args = flags.Args()

	if len(args) > 1 {
		c.UI.Error(fmt.Sprintf("Too many arguments (expected at most 1, got %d)", len(args)))
		return 1
	}

	client, err := c.Client()
	if err != nil {
		c.UI.Error(err.Error())
		return 2
	}

	// var (
	// 	partialPath string
	// 	v2          bool
	// )

	// In this case, this arg is a path-like combination of mountPath/secretPath.
	// (e.g. "secret/foo")
	// partialPath = sanitizePath(args[0])
	// mountPath, v2, err = isKVv2(partialPath, client)
	// if err != nil {
	// 	c.UI.Error(err.Error())
	// 	return 2
	// }

	/*
		// Add /data to v2 paths only
		var fullPath string
		if v2 {
			fullPath = addPrefixToKVPath(partialPath, mountPath, "data")
			data = map[string]interface{}{
				"data":    data,
				"options": map[string]interface{}{},
			}

			if c.flagCAS > -1 {
				data["options"].(map[string]interface{})["cas"] = c.flagCAS
			}
		} else {
			// v1
			if mountFlagSyntax {
				fullPath = path.Join(mountPath, partialPath)
			} else {
				fullPath = partialPath
			}
		}

		secret, err := client.Logical().Write(fullPath, data)
		if err != nil {
			c.UI.Error(fmt.Sprintf("Error writing data to %s: %s", fullPath, err))
			if secret != nil {
				OutputSecret(c.UI, secret)
			}
			return 2
		}
		if secret == nil {
			// Don't output anything unless using the "table" format
			if Format(c.UI) == "table" {
				c.UI.Info(fmt.Sprintf("Success! Data written to: %s", fullPath))
			}
			return 0
		}

		if c.flagField != "" {
			return PrintRawField(c.UI, secret, c.flagField)
		}

		if Format(c.UI) == "table" {
			outputPath(c.UI, fullPath, "Secret Path")
			metadata := secret.Data
			c.UI.Info(getHeaderForMap("Metadata", metadata))
			return OutputData(c.UI, metadata)
		}

		return OutputSecret(c.UI, secret)
	*/

	var execCommand string
	if c.flagExec != "" {
		execCommand = c.flagExec
	} else {
		execCommand = "env"
	}

	agentConfig := config.ConfigGen{
		Vault: &config.VaultGen{
			Address: client.Address(),
		},
		AutoAuth: &config.AutoAuthGen{
			Method: &config.AutoAuthMethodGen{
				Type: "token_file",
				Config: config.AutoAuthMethodConfigGen{
					TokenFilePath: "$HOME/.vault-token",
				},
			},
		},
		Exec: &config.ExecConfig{
			Command:            execCommand,
			Args:               []string{},
			RestartOnNewSecret: "always",
			RestartKillSignal:  "SIGTERM",
		},
	}

	var configPath string
	if len(args) == 1 {
		configPath = args[0]
	} else {
		configPath = "agent.hcl"
	}

	contents := hclwrite.NewEmptyFile()

	gohcl.EncodeIntoBody(&agentConfig, contents.Body())

	f, err := os.Create(configPath)
	if err != nil {
		c.UI.Error(fmt.Sprintf("Could not create configuration file %q: %v", configPath, err.Error()))
		return 1
	}
	defer func() {
		if err := f.Close(); err != nil {
			c.UI.Error(fmt.Sprintf("Could not close configuration file %q: %v", configPath, err.Error()))
		}
	}()

	if _, err := contents.WriteTo(f); err != nil {
		c.UI.Error(fmt.Sprintf("Could not write to configuration file %q: %v", configPath, err.Error()))
		return 1
	}

	c.UI.Info(fmt.Sprintf("Successfully generated %q configuration file!", configPath))

	return 0
}
