package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"text/template"

	"code.vegaprotocol.io/vegacapsule/generator/datanode"
	"code.vegaprotocol.io/vegacapsule/generator/tendermint"
	"code.vegaprotocol.io/vegacapsule/generator/vega"
	"code.vegaprotocol.io/vegacapsule/generator/visor"
	"code.vegaprotocol.io/vegacapsule/state"
	"code.vegaprotocol.io/vegacapsule/types"
	"github.com/spf13/cobra"
)

var (
	nodeSetsGroupsNames []string
	nodeSetsNames       []string
	nodeSetTemplateType string

	nodeSetTemplateTypes = []templateKindType{
		vegaNodeSetTemplateType,
		tendermintNodeSetTemplateType,
		dataNodeNodeSetTemplateType,
		visorRunNodeSetTemplateType,
	}
)

var templateNodeSetsCmd = &cobra.Command{
	Use:   "node-sets",
	Short: "Run config templating for Vega, Tendermit, DataNode, Visor node sets",
	RunE: func(cmd *cobra.Command, args []string) error {
		template, err := os.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template %q: %w", templatePath, err)
		}

		networkState, err := state.LoadNetworkState(homePath)
		if err != nil {
			return fmt.Errorf("failed to load network state: %w", err)
		}

		if networkState.Empty() {
			return networkNotBootstrappedErr("template node-sets")
		}

		return templateNodeSets(templateKindType(nodeSetTemplateType), string(template), networkState)
	},
	SilenceUsage: true,
	Example: `
# Generate the vega template
vegacapsule template node-sets --type vega --path .../vega_validators.tmpl--nodeset-group-name validators

# Generate the tendermint template
vegacapsule template node-sets --type tendermint --path .../tendermint_validators.tmpl --nodeset-group-name validators

# Generate the vega template
vegacapsule template node-sets --type data-node --path .../data-node_full.tmpl --nodeset-group-name full

# Generate the configuration for multiple node sets #1
vegacapsule template node-sets --type vega --path .../vega_common.tmpl --nodeset-group-name validators,full

# Generate the configuration for multiple node sets #2
vegacapsule template node-sets --type vega --path .../vega_common.tmpl --nodeset-group-name validators --nodeset-group-name full

# Generate the configuration for particular vega node
vegacapsule template node-sets --type vega --path .../vega_validator.tmpl --nodeset-name testnet-nodeset-validators-0-validator

# Update prebiously generated network configuration with merge to current configuration
main.go template node-sets --type vega --path .../vega_common.tmpl --nodeset-group-name validators,full --with-merge --update-network`,
}

func init() {
	templateNodeSetsCmd.PersistentFlags().StringVar(&nodeSetTemplateType,
		"type",
		"",
		fmt.Sprintf("Template type, one of: %v", nodeSetTemplateTypes),
	)

	templateNodeSetsCmd.PersistentFlags().BoolVar(&withMerge,
		"with-merge",
		false,
		"Defines whether the templated config should be merged with the originally initiated one",
	)

	templateNodeSetsCmd.PersistentFlags().StringSliceVar(&nodeSetsGroupsNames,
		"nodeset-group-name",
		[]string{},
		"Allows to apply template to all node sets in a specific groups, Flag takes a coma separated list of strings.",
	)

	templateNodeSetsCmd.PersistentFlags().StringSliceVar(&nodeSetsNames,
		"nodeset-name",
		[]string{},
		"Allows to apply template to a specific node sets. Flag takes a coma separated list of strings",
	)

	templateNodeSetsCmd.MarkPersistentFlagRequired("type") // nolint:errcheck
}

type templateFunc func(ns types.NodeSet, tmpl *template.Template) (*bytes.Buffer, error)

func templateNodeSets(tmplType templateKindType, templateRaw string, netState *state.NetworkState) error {
	nodeSets, err := filterNodesSets(netState, nodeSetsNames, nodeSetsGroupsNames)
	if err != nil {
		return err
	}

	switch tmplType {
	case tendermintNodeSetTemplateType:
		tmpl, err := tendermint.NewConfigTemplate(templateRaw)
		if err != nil {
			return err
		}

		gen, err := tendermint.NewConfigGenerator(netState.Config, netState.GeneratedServices.NodeSets.ToSlice())
		if err != nil {
			return err
		}

		return templateNodeSetConfig(gen.TemplateConfig, gen.TemplateAndMergeConfig, tmplType, tmpl, nodeSets)
	case vegaNodeSetTemplateType:
		tmpl, err := vega.NewConfigTemplate(templateRaw)
		if err != nil {
			return err
		}

		gen, err := vega.NewConfigGenerator(netState.Config)
		if err != nil {
			return err
		}

		templateF := func(ns types.NodeSet, tmpl *template.Template) (*bytes.Buffer, error) {
			return gen.TemplateConfig(ns, netState.GeneratedServices.Faucet, tmpl)
		}

		templateAndMergeF := func(ns types.NodeSet, tmpl *template.Template) (*bytes.Buffer, error) {
			return gen.TemplateAndMergeConfig(ns, netState.GeneratedServices.Faucet, tmpl)
		}

		return templateNodeSetConfig(templateF, templateAndMergeF, tmplType, tmpl, nodeSets)
	case dataNodeNodeSetTemplateType:
		tmpl, err := datanode.NewConfigTemplate(templateRaw)
		if err != nil {
			return err
		}

		gen, err := datanode.NewConfigGenerator(netState.Config, netState.GeneratedServices.NodeSets.ToSlice())
		if err != nil {
			return err
		}

		return templateNodeSetConfig(gen.TemplateConfig, gen.TemplateAndMergeConfig, tmplType, tmpl, nodeSets)
	case visorRunNodeSetTemplateType:
		tmpl, err := visor.NewConfigTemplate(templateRaw)
		if err != nil {
			return err
		}

		gen, err := visor.NewGenerator(netState.Config)
		if err != nil {
			return err
		}

		return templateNodeSetConfig(gen.TemplateConfig, gen.TemplateAndMergeConfig, tmplType, tmpl, nodeSets)
	}

	return fmt.Errorf("template type %q does not exists", tmplType)
}

func templateNodeSetConfig(
	templateF, templateAndMergeF templateFunc,
	tmplType templateKindType,
	template *template.Template,
	nodeSets []types.NodeSet,
) error {
	var buff *bytes.Buffer
	var err error

	for _, ns := range nodeSets {
		if withMerge {
			buff, err = templateAndMergeF(ns, template)
		} else {
			buff, err = templateF(ns, template)
		}
		if err != nil {
			return err
		}

		if templateUpdateNetwork {
			if err := updateTemplateForNode(tmplType, ns, buff); err != nil {
				return fmt.Errorf("failed to update template for node %d: %w", ns.Index, err)
			}
		} else {
			fileName := fmt.Sprintf("%s-%s.conf", tmplType, ns.Name)

			if err := outputTemplate(buff, path.Join(templateOutDir, fileName), true); err != nil {
				return fmt.Errorf("failed to print generated template for node %d: %w", ns.Index, err)
			}
		}
	}

	return nil
}

func filterNodesSets(netState *state.NetworkState, nodeSetsNames, nodeSetsGroupsNames []string) ([]types.NodeSet, error) {
	if len(nodeSetsGroupsNames) == 0 && len(nodeSetsNames) == 0 {
		return nil, fmt.Errorf("either of 'nodeset-name', 'nodeset-group-name' flags must be defined to template node set")
	}

	filters := []types.NodeSetFilter{}
	if len(nodeSetsNames) > 0 {
		filters = append(filters, types.NodeSetFilterByNames(nodeSetsNames))
	}

	if len(nodeSetsGroupsNames) > 0 {
		filters = append(filters, types.NodeSetFilterByGroupNames(nodeSetsGroupsNames))
	}

	nodeSets := types.FilterNodeSets(netState.GeneratedServices.NodeSets.ToSlice(), filters...)
	if len(nodeSets) == 0 {
		return nil, fmt.Errorf("node set group with given criteria [names: '%v', groups-names: '%v'] not found", nodeSetsNames, nodeSetsGroupsNames)
	}

	return nodeSets, nil
}
