package main

import (
	"bytes"
	_ "embed"
	"github.com/stretchr/testify/assert"
	"testing"
)

//go:embed "testdata/plain.txt"
var plainOutput string

//go:embed "testdata/metrics.csv"
var csvOutput string

func Test_single(t *testing.T) {

	for _, tt := range []struct {
		format   string
		expected string
	}{
		{format: "plain", expected: plainOutput},
		{format: "csv", expected: csvOutput},
	} {
		t.Run(tt.format, func(t *testing.T) {
			var buff bytes.Buffer
			out = &buff

			err := single("testdata/metrics-1", &metricOptions{
				minHistogramCount: 5,
				trimPrefix:        "rox_central_",
				format:            tt.format,
			})

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, buff.String(), buff.String())
		})
	}

}
