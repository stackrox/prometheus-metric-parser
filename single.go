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
			if opts.format == "gcloud" && opts.projectID == "" {
				return errors.New("a --project-id must be specified for gcloud")
			}
			if (opts.format == "gcloud" || opts.format == "influxdb") && opts.timestamp == 0 {
				return errors.New("a --timestamp must be specified for gcloud/influxdb ingest")
			}
			families, err := readFile(file)
			if err != nil {
				return err
			}
			if opts.format == "gcloud" {
				gcloud, err := gcloudConnect(opts.projectID)
				if err != nil {
					log.Fatalf("Cannot connect to gcloud: %v\n", err)
				}
				gcloud.createGcloudMetricDescriptors(families)
				gcloud.close()
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
