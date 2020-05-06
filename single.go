package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func singleCommand() *cobra.Command {
	var (
		file string

		opts *metricOptions
	)

	c := &cobra.Command{
		Use:   "single",
		Short: "Single takes all the metrics and outputs them as key value pairs. It will average histograms",
		RunE: func(c *cobra.Command, _ []string) error {
			if file == "" {
				return errors.New("file must be specified")
			}
			families, err := readFile(file)
			if err != nil {
				return err
			}
			metricMap, err := familiesToKeyPairs(families, opts)
			if err != nil {
				return err
			}
			metricMap.Print()

			return nil
		},
	}

	c.Flags().StringVar(&file, "file", "", "file to parse")

	opts = addMetricFlags(c)
	return c
}
