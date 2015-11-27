package prometheus

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/golang/protobuf/proto"

	dto "github.com/prometheus/client_model/go"
)

var (
	metricNameRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_:]*$`)
	labelNameRE  = regexp.MustCompile("^[a-zA-Z_][a-zA-Z0-9_]*$")
)

// reservedLabelPrefix is a prefix which is not legal in user-supplied
// label names.
const reservedLabelPrefix = "__"

// Labels represents a collection of label name -> value mappings. This type is
// commonly used with the With(Labels) and GetMetricWith(Labels) methods of
// metric vector Collectors, e.g.:
//     myVec.With(Labels{"code": "404", "method": "GET"}).Add(42)
//
// The other use-case is the specification of constant label pairs in Opts or to
// create a Desc.
type Labels map[string]string

// Desc is used to describe the meta-data of a Metric (via its Desc method) or a
// group of metrics collected by a Collector (via its Describe method). Some of its
// methods are only used internally and are therefore not exported, which also
// prevents users to implement their own descriptors. Descriptor instances must
// be created via suitable NewXXXDesc functions and will in generally only be
// needed in custom collectors.
//
// Desc implementations are immutable by contract.
//
// A Desc that also implements the error interface is called an invalid
// descriptor, which is solely used to communicate an error and must never be
// processed further.
type Desc interface {
	// String returns a string representation of the descriptor as usual. It
	// is also used as an ID that must be unique among all descriptors
	// registered by a Registry.
	String() string
	dims() string
}

// NewInvalidDesc returns a descriptor that also implements the error
// interface. It is used to communicate an error during a call of Desc (Metric
// method) or Describe (Collector method). Create with NewInvalidDesc.
func NewInvalidDesc(err error) Desc {
	return &invalidDesc{err: err}
}

type invalidDesc struct {
	err error
}

func (d *invalidDesc) Error() string {
	return d.err.Error()
}

func (d *invalidDesc) String() string {
	return "[invalid] " + d.err.Error()
}

func (d *invalidDesc) dims() string {
	return ""
}

// NewPrefixDesc returns a descriptor that is used by Collectors that want to
// reserve a whole metric name prefix for their own use. An invalid descriptor
// is returned if the prefix is not a valid as the start of a metric
// name. However, an empty prefix is valid and reserves all metric names.
func NewPrefixDesc(prefix string) Desc {
	if prefix != "" && !validMetricName(prefix) {
		return NewInvalidDesc(fmt.Errorf("%q is not a valid metric name prefix", prefix))
	}
	return &prefixDesc{pfx: prefix}
}

type prefixDesc struct {
	prefix string
}

func (d *prefixDesc) String() string {
	return "[prefix] " + d.prefix
}

func (d *prefixDesc) dims() string {
	return "" // PrefixDesc is for all dimensions.
}

func (d *prefixDesc) overlapsWith(other Desc) bool {
	switch o := other.(type) {
	case *invalidDesc:
		// Invalid descs never overlap.
		return false
	case *partialDesc, *fullDesc:
		return strings.HasPrefix(o.fqName, d.prefix)
	case *prefixDesc:
		return strings.HasPrefix(o.prefix, d.prefix) || strings.HasPrefix(d.Prefix, o.prefix)
	default:
		panic(fmt.Errorf("unexpected type of descriptor %q", o))
	}
}

// NewPartialDesc returns a descriptor that is used by Collectors that want to
// reserve a specific metric name and type with specific label dimensions of
// which some (but not all) label values might be set already. An invalid
// descriptor is returned in the following cases: The resulting label name
// (assembled from namespace, subsystem, and name) is invalid, the help string
// is empty, unsetLabels is empty or contains invalid label names,
// setLabels (which might be empty) contains invalid label names or label
// values, metricType is not a valid MetricType.
func NewPartialDesc(
	namespace, subsystem, name string,
	metricType MetricType,
	help string,
	setLabels Labels,
	unsetLabels []string,
) Desc {
	return nil // TODO
}

// NewFullDesc returns a descriptor with fully specified name, type, and
// labels. It can be used by Collectors and must be used by Metrics. An invalid
// descriptor is returned if the resulting label name (assembled from namespace,
// subsystem, and name) is invalid, the help string is empty, metricType has an
// invalid value, or the labels contain invalid label names or values. The labels
// might be empty, though.
func NewFullDesc(
	namespace, subsystem, name string,
	metricType MetricType,
	help string,
	labels Labels,
) Desc {
	return nil // TODO
}

// FullySpecify returns a fully specified descriptor based on the provided
// partial descriptor by setting all the unset labels to the provided values (in
// the same order as they have been specified in the NewFullDesc or DeSpecify
// call). An invalid desc is returned if the provided desc is not a partial
// descriptor, the cardinality of labelValues does not fit, or labelValues
// contains invalid label values.
func FullySpecify(desc Desc, labelValues ...string) Desc {
	d, ok := desc.(*partialDesc)
	if !ok {
		return NewInvalidDesc(fmt.Errorf("tried to fully specify non-partial descriptor %q", desc))
	}
	return nil // TODO
}

// DeSpecify creates a partial descriptor based on the provided full descriptor
// by adding un-set labels with the provided label names. An invalid desc is
// returned if the provided desc is not a full descriptor, or labelNames
// contains invalid label names (or no label names at all).
func DeSpecify(desc Desc, labelNames ...string) Desc {
	d, ok := desc.(*fullDesc)
	if !ok {
		return NewInvalidDesc(fmt.Errorf("tried to de-specify non-full descriptor %q", desc))
	}
	if len(ln) == 0 {
		return NewInvalidDesc(fmt.Errorf("no label names provided to de-specify %q", desc))
	}
	for _, ln := range labelNames {
		if !validLabelName(ln) {
			return NewInvalidDesc(fmt.Errorf("encountered invalid label name %q while de-specifying %q", ln, desc))
		}
	}
	return &partialDesc{*d, labelNames}
}

type fullDesc struct {
	fqName, help string
	metricType   MetricType
	setLabels    []*dto.LabelPair // Sorted.
}

type partialDesc struct {
	fullDesc
	unsetLabels []string // Keep in original order.
}

// buildFQName joins the given three name components by "_". Empty name
// components are ignored. If the name parameter itself is empty, an empty
// string is returned, no matter what.
func buildFQName(namespace, subsystem, name string) string {
	if name == "" {
		return ""
	}
	switch {
	case namespace != "" && subsystem != "":
		return namespace + "_" + subsystem + "_" + name
	case namespace != "":
		return namespace + "_" + name
	case subsystem != "":
		return subsystem + "_" + name
	}
	return name
}

func validMetricName(n string) bool {
	return metricNameRE.MatchString(n)
}

func validLabelName(l string) bool {
	return labelNameRE.MatchString(l) &&
		!strings.HasPrefix(l, reservedLabelPrefix)
}

func validLabelValue(l string) bool {
	return utf8.ValidString(l)
}

// OLD CODE below.

// Desc is the descriptor used by every Prometheus Metric. It is essentially
// the immutable meta-data of a Metric. The normal Metric implementations
// included in this package manage their Desc under the hood. Users only have to
// deal with Desc if they use advanced features like the ExpvarCollector or
// custom Collectors and Metrics.
//
// Descriptors registered with the same registry have to fulfill certain
// consistency and uniqueness criteria if they share the same fully-qualified
// name: They must have the same help string and the same label names (aka label
// dimensions) in each, constLabels and variableLabels, but they must differ in
// the values of the constLabels.
//
// Descriptors that share the same fully-qualified names and the same label
// values of their constLabels are considered equal.
//
// Use NewDesc to create new Desc instances.
type Desc struct {
	// fqName has been built from Namespace, Subsystem, and Name.
	fqName string
	// help provides some helpful information about this metric.
	help string
	// constLabelPairs contains precalculated DTO label pairs based on
	// the constant labels.
	constLabelPairs []*dto.LabelPair
	// VariableLabels contains names of labels for which the metric
	// maintains variable values.
	variableLabels []string
	// id is a hash of the values of the ConstLabels and fqName. This
	// must be unique among all registered descriptors and can therefore be
	// used as an identifier of the descriptor.
	id uint64
	// dimHash is a hash of the label names (preset and variable) and the
	// Help string. Each Desc with the same fqName must have the same
	// dimHash.
	dimHash uint64
	// err is an error that occured during construction. It is reported on
	// registration time.
	err error
}

// NewDesc allocates and initializes a new Desc. Errors are recorded in the Desc
// and will be reported on registration time. variableLabels and constLabels can
// be nil if no such labels should be set. fqName and help must not be empty.
//
// variableLabels only contain the label names. Their label values are variable
// and therefore not part of the Desc. (They are managed within the Metric.)
//
// For constLabels, the label values are constant. Therefore, they are fully
// specified in the Desc. See the Opts documentation for the implications of
// constant labels.
func NewDesc(fqName, help string, variableLabels []string, constLabels Labels) *Desc {
	d := &Desc{
		fqName:         fqName,
		help:           help,
		variableLabels: variableLabels,
	}
	if help == "" {
		d.err = errors.New("empty help string")
		return d
	}
	if !metricNameRE.MatchString(fqName) {
		d.err = fmt.Errorf("%q is not a valid metric name", fqName)
		return d
	}
	// labelValues contains the label values of const labels (in order of
	// their sorted label names) plus the fqName (at position 0).
	labelValues := make([]string, 1, len(constLabels)+1)
	labelValues[0] = fqName
	labelNames := make([]string, 0, len(constLabels)+len(variableLabels))
	labelNameSet := map[string]struct{}{}
	// First add only the const label names and sort them...
	for labelName := range constLabels {
		if !validLabelName(labelName) {
			d.err = fmt.Errorf("%q is not a valid label name", labelName)
			return d
		}
		labelNames = append(labelNames, labelName)
		labelNameSet[labelName] = struct{}{}
	}
	sort.Strings(labelNames)
	// ... so that we can now add const label values in the order of their names.
	for _, labelName := range labelNames {
		labelValues = append(labelValues, constLabels[labelName])
	}
	// Now add the variable label names, but prefix them with something that
	// cannot be in a regular label name. That prevents matching the label
	// dimension with a different mix between preset and variable labels.
	for _, labelName := range variableLabels {
		if !validLabelName(labelName) {
			d.err = fmt.Errorf("%q is not a valid label name", labelName)
			return d
		}
		labelNames = append(labelNames, "$"+labelName)
		labelNameSet[labelName] = struct{}{}
	}
	if len(labelNames) != len(labelNameSet) {
		d.err = errors.New("duplicate label names")
		return d
	}
	vh := hashNew()
	for _, val := range labelValues {
		vh = hashAdd(vh, val)
		vh = hashAddByte(vh, separatorByte)
	}
	d.id = vh
	// Sort labelNames so that order doesn't matter for the hash.
	sort.Strings(labelNames)
	// Now hash together (in this order) the help string and the sorted
	// label names.
	lh := hashNew()
	lh = hashAdd(lh, help)
	lh = hashAddByte(lh, separatorByte)
	for _, labelName := range labelNames {
		lh = hashAdd(lh, labelName)
		lh = hashAddByte(lh, separatorByte)
	}
	d.dimHash = lh

	d.constLabelPairs = make([]*dto.LabelPair, 0, len(constLabels))
	for n, v := range constLabels {
		d.constLabelPairs = append(d.constLabelPairs, &dto.LabelPair{
			Name:  proto.String(n),
			Value: proto.String(v),
		})
	}
	sort.Sort(LabelPairSorter(d.constLabelPairs))
	return d
}

func (d *Desc) String() string {
	lpStrings := make([]string, 0, len(d.constLabelPairs))
	for _, lp := range d.constLabelPairs {
		lpStrings = append(
			lpStrings,
			fmt.Sprintf("%s=%q", lp.GetName(), lp.GetValue()),
		)
	}
	return fmt.Sprintf(
		"Desc{fqName: %q, help: %q, constLabels: {%s}, variableLabels: %v}",
		d.fqName,
		d.help,
		strings.Join(lpStrings, ","),
		d.variableLabels,
	)
}
