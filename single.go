package main

import (
	"errors"
	"log"

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
			if opts.format == "gcp-monitoring" && opts.projectID == "" {
				return errors.New("a --project-id must be specified for gcp-monitoring")
			}
			if (opts.format == "gcp-monitoring" || opts.format == "influxdb") && opts.timestamp == 0 {
				return errors.New("a --timestamp must be specified for gcp-monitoring/influxdb ingest")
			}
			families, err := readFile(file)
			if err != nil {
				return err
			}
			if opts.format == "gcp-monitoring" {
				gcpMonitoring, err := gcpMonitoringConnect(opts.projectID)
				defer gcpMonitoring.close()
				if err != nil {
					log.Fatalf("Cannot connect to GCP monitoring: %v\n", err)
				}
				gcpMonitoring.createMetricDescriptors(families)
			}
			metricMap, err := familiesToKeyPairs(families, opts)
			if err != nil {
				return err
			}
			metricMap.Print(opts.format, labelsFromOpts(opts.labels), opts.projectID, opts.timestamp)

			return nil
		},
	}

	c.Flags().StringVar(&file, "file", "", "file to parse")

	opts = addMetricFlags(c)
	return c
}
