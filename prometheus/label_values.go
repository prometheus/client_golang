package prometheus

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

func validateLabelValues(vals []string, expectedNumberOfValues int) error {
	if len(vals) != expectedNumberOfValues {
		return errInconsistentCardinality
	}

	for _, val := range vals {
		if !utf8.ValidString(val) {
			return errors.New(fmt.Sprintf("label value %#v is not valid utf8", val))
		}
	}

	return nil
}

func validateLabels(labels Labels, expectedNumberOfValues int) error {
	if len(labels) != expectedNumberOfValues {
		return errInconsistentCardinality
	}

	for name, val := range labels {
		if !utf8.ValidString(val) {
			return errors.New(fmt.Sprintf("label %s: %#v is not valid utf8", name, val))
		}
	}

	return nil
}
