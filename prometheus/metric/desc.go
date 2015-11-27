
import "fmt"

func NewDesc(
	namespace, subsystem, name string,
	help string,
	constLabels Labels,
	variableLabels []string,
) Desc {
	fqName := buildFQName(namespace, subsystem, name)
	if !metricNameRE.MatchString(fqName) {
		return NewInvalidDesc(fmt.Errorf("%q is not a valid metric name", fqName))
	}
	if help == "" {
		return NewInvalidDesc(fmt.Errorf("empty help string for metric %q", fqName))
	}

	return nil // TODO
}

type regularDesc struct {
	baseDesc
	fqName, help    string
	constLabelPairs []*dto.LabelPair
	variableLabels  []string
}

type prefixDesc struct {
	baseDesc
	prefix string
}

type Set struct {
	regular map[string]*regularDesc
	// The prefix ones should be tries. But it's unlikely to have many of them.
	prefix []*prefixDesc
}

func (s *Set) Add(d Desc) error {
	if d.Error() != nil {
		return d.Error()
	}
	return nil
}

func (s *Set) Remove(d Desc) bool {
	return false
}

