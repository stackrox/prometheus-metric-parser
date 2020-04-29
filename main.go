package main

import (
	"bytes"
	"io/ioutil"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"
	"github.com/spf13/cobra"

	"log"
	"os"
)

func main() {
	c := &cobra.Command{
		SilenceUsage: true,
		Use:          os.Args[0],
	}

	c.AddCommand(
		singleCommand(),
		compareCommand(),
	)

	if err := c.Execute(); err != nil {
		os.Exit(1)
	}
}

func readFile(path string) ([]*prom2json.Family, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	mfChan := make(chan *dto.MetricFamily, 1024)
	go func() {
		if err := prom2json.ParseReader(bytes.NewReader(data), mfChan); err != nil {
			log.Fatal("error reading metrics:", err)
		}
	}()

	var result []*prom2json.Family
	for mf := range mfChan {
		result = append(result, prom2json.NewFamily(mf))
	}
	return result, nil
}
