// Copyright 2014 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheus

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-multierror"

	dto "github.com/prometheus/client_model/go"
)

const (
	// Capacity for the channel to collect metrics and descriptors.
	capMetricChan = 1000
	capDescChan   = 10
)

// DefaultRegistry is a Registry instance that has a ProcessCollector and a
// GoCollector pre-registered. DefaultRegisterer and DefaultDeliverer are both
// pointing to it. A number of convenience functions in this package act on
// them. This approach to keep a default instance as global state mirrors the
// approach of other packages in the Go standard library. Note that there are
// caveats. Change the variables with caution and only if you understand the
// consequences. Users who want to avoid global state altogether should not
// use the convenience function and act on custom instances instead.
var (
	DefaultRegistry              = NewRegistry()
	DefaultRegisterer Registerer = DefaultRegistry
	DefaultDeliverer  Deliverer  = DefaultRegistry
)

func init() {
	MustRegister(NewProcessCollector(os.Getpid(), ""))
	MustRegister(NewGoCollector())
}

// NewRegistry creates a new vanilla Registry without any Collectors
// pre-registered.
func NewRegistry() *Registry {
	return &Registry{
		collectorsByID:  map[uint64]Collector{},
		descIDs:         map[uint64]struct{}{},
		dimHashesByName: map[string]uint64{},
	}
}

// NewPedanticRegistry returns a registry that checks during collection if each
// collected Metric is consistent with its reported Desc, and if the Desc has
// actually been registered with the registry.
//
// Usually, a Registry will be happy as long as the union of all collected
// Metrics is consistent and valid even if some metrics are not consistent with
// their own Desc or with one of the Descs provided by their
// Collector. Well-behaved Collectors and Metrics will only provide consistent
// Descs. This Registry is useful to test the implementation of Collectors and
// Metrics.
func NewPedanticRegistry() *Registry {
	r := NewRegistry()
	r.pedanticChecksEnabled = true
	return r
}

// Registerer is the interface for the part of a registry in charge of
// registering and unregistering. Users of custom registries should use
// Registerer as type for registration purposes (rather then the Registry type
// directly). In that way, they are free to exchange the Registerer
// implementation (e.g. for testing purposes).
type Registerer interface {
	// Register registers a new Collector to be included in metrics
	// collection. It returns an error if the descriptors provided by the
	// Collector are invalid or if they - in combination with descriptors of
	// already registered Collectors - do not fulfill the consistency and
	// uniqueness criteria described in the documentation of metric.Desc.
	//
	// If the provided Collector is equal to a Collector already registered
	// (which includes the case of re-registering the same Collector), the
	// returned error is an instance of AlreadyRegisteredError, which
	// contains the previously registered Collector.
	//
	// It is in general not safe to register the same Collector multiple
	// times concurrently.
	Register(Collector) error
	// MustRegister works like Register but registers any number of
	// Collectors and panics upon the first registration that causes an
	// error.
	MustRegister(...Collector)
	// Unregister unregisters the Collector that equals the Collector passed
	// in as an argument.  (Two Collectors are considered equal if their
	// Describe method yields the same set of descriptors.) The function
	// returns whether a Collector was unregistered.
	//
	// Note that even after unregistering, it will not be possible to
	// register a new Collector that is inconsistent with the unregistered
	// Collector, e.g. a Collector collecting metrics with the same name but
	// a different help string. The rationale here is that the same registry
	// instance must only collect consistent metrics throughout its
	// lifetime.
	Unregister(Collector) bool
}

// Deliverer is the interface for the part of a registry in charge of delivering
// the collected metrics, wich the same general implication as described for the
// Registerer interface.
type Deliverer interface {
	// Deliver collects metrics from registered Collectors and returns them
	// as lexicographically sorted MetricFamily protobufs. Even if an error
	// occurs, Deliver attempts to collect as many metrics as
	// possible. Hence, if a non-nil error is returned, the returned
	// MetricFamily slice could be nil (in case of a fatal error that
	// prevented any meaningful metric collection) or contain a number of
	// MetricFamily protobufs, some of which might be incomplete, and some
	// might be missing altogether. The returned error (which might be a
	// multierror.Error) explains the details. In any case, the MetricFamily
	// protobufs are consistent and valid for Prometheus to ingest (e.g. no
	// duplicate metrics, no invalid identifiers). In scenarios where
	// complete collection is critical, the returned MetricFamily protobufs
	// should be disregarded if the returned error is non-nil.
	Deliver() ([]*dto.MetricFamily, error)
}

// Register registers the provided Collector with the DefaultRegisterer.
//
// Register is a shortcut for DefaultRegisterer.Register(c). See there for more
// details.
func Register(c Collector) error {
	return DefaultRegisterer.Register(c)
}

// MustRegister registers the provided Collectors with the DefaultRegisterer and
// panics if any error occurs.
//
// MustRegister is a shortcut for DefaultRegisterer.MustRegister(cs...). See
// there for more details.
func MustRegister(cs ...Collector) {
	DefaultRegisterer.MustRegister(cs...)
}

// RegisterOrGet registers the provided Collector with the DefaultRegisterer and
// returns the Collector, unless an equal Collector was registered before, in
// which case that Collector is returned.
//
// Deprecated: RegisterOrGet is merely a convenience function for the
// implementation as described in the documentation for
// AlreadyRegisteredError. As the use case is relatively rare, this function
// will be removed in a future version of this package to clean up the
// namespace.
func RegisterOrGet(c Collector) (Collector, error) {
	if err := Register(c); err != nil {
		if are, ok := err.(AlreadyRegisteredError); ok {
			return are.ExistingCollector, nil
		}
		return nil, err
	}
	return c, nil
}

// MustRegisterOrGet behaves like RegisterOrGet but panics instead of returning
// an error.
//
// Deprecated: This is deprecated for the same reason RegisterOrGet is. See
// there for details.
func MustRegisterOrGet(c Collector) Collector {
	c, err := RegisterOrGet(c)
	if err != nil {
		panic(err)
	}
	return c
}

// Unregister removes the registration of the provided Collector from the
// DefaultRegisterer.
//
// Unregister is a shortcut for DefaultRegisterer.Unregister(c). See there for
// more details.
func Unregister(c Collector) bool {
	return DefaultRegisterer.Unregister(c)
}

// SetMetricFamilyInjectionHook sets a MetricFamily injection hook on the
// DefaultRegistry.
//
// It's a shortcut for DefaultRegistry.SetInjectionHook(hook). See there for
// more details.
//
// Deprecated: In the rare cases this call is needed, users should simply call
// DefaultRegistry.SetInjectionHook directly.
func SetMetricFamilyInjectionHook(hook func() []*dto.MetricFamily) {
	DefaultRegistry.SetInjectionHook(hook)
}

// AlreadyRegisteredError is returned by the Registerer.Register if the
// Collector to be registered has already been registered before, or a different
// Collector that collects the same metrics has been registered
// before. Registration fails in that case, but you can detect from the kind of
// error what has happened. The error contains fields for the existing Collector
// and the (rejected) new Collector that equals the existing one. This can be
// used in the following way:
//
//	reqCounter := prometheus.NewCounter( /* ... */ )
//	if err := registry.Register(reqCounter); err != nil {
//		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
//			// A counter for that metric has been registered before.
//			// Use the old counter from now on.
//			reqCounter = are.ExistingCollector.(prometheus.Counter)
//		} else {
//			// Something else went wrong!
//			panic(err)
//		}
//	}
type AlreadyRegisteredError struct {
	ExistingCollector, NewCollector Collector
}

func (err AlreadyRegisteredError) Error() string {
	return "duplicate metrics collector registration attempted"
}

// Registry registers Prometheus collectors, collects their metrics, and
// delivers them for exposition. It implements Registerer and Deliverer. The
// zero value is not usable. Use NewRegistry or NewPedanticRegistry to create
// instances.
type Registry struct {
	mtx                       sync.RWMutex
	collectorsByID            map[uint64]Collector // ID is a hash of the descIDs.
	descIDs                   map[uint64]struct{}
	dimHashesByName           map[string]uint64
	metricFamilyInjectionHook func() []*dto.MetricFamily
	pedanticChecksEnabled     bool
}

// Register implements Registerer.
func (r *Registry) Register(c Collector) error {
	var (
		descChan           = make(chan *Desc, capDescChan)
		newDescIDs         = map[uint64]struct{}{}
		newDimHashesByName = map[string]uint64{}
		collectorID        uint64 // Just a sum of all desc IDs.
		duplicateDescErr   error
	)
	go func() {
		c.Describe(descChan)
		close(descChan)
	}()
	r.mtx.Lock()
	defer r.mtx.Unlock()
	// Coduct various tests...
	for desc := range descChan {

		// Is the descriptor valid at all?
		if desc.err != nil {
			return fmt.Errorf("descriptor %s is invalid: %s", desc, desc.err)
		}

		// Is the descID unique?
		// (In other words: Is the fqName + constLabel combination unique?)
		if _, exists := r.descIDs[desc.id]; exists {
			duplicateDescErr = fmt.Errorf("descriptor %s already exists with the same fully-qualified name and const label values", desc)
		}
		// If it is not a duplicate desc in this collector, add it to
		// the collectorID.  (We allow duplicate descs within the same
		// collector, but their existence must be a no-op.)
		if _, exists := newDescIDs[desc.id]; !exists {
			newDescIDs[desc.id] = struct{}{}
			collectorID += desc.id
		}

		// Are all the label names and the help string consistent with
		// previous descriptors of the same name?
		// First check existing descriptors...
		if dimHash, exists := r.dimHashesByName[desc.fqName]; exists {
			if dimHash != desc.dimHash {
				return fmt.Errorf("a previously registered descriptor with the same fully-qualified name as %s has different label names or a different help string", desc)
			}
		} else {
			// ...then check the new descriptors already seen.
			if dimHash, exists := newDimHashesByName[desc.fqName]; exists {
				if dimHash != desc.dimHash {
					return fmt.Errorf("descriptors reported by collector have inconsistent label names or help strings for the same fully-qualified name, offender is %s", desc)
				}
			} else {
				newDimHashesByName[desc.fqName] = desc.dimHash
			}
		}
	}
	// Did anything happen at all?
	if len(newDescIDs) == 0 {
		return errors.New("collector has no descriptors")
	}
	if existing, exists := r.collectorsByID[collectorID]; exists {
		return AlreadyRegisteredError{
			ExistingCollector: existing,
			NewCollector:      c,
		}
	}
	// If the collectorID is new, but at least one of the descs existed
	// before, we are in trouble.
	if duplicateDescErr != nil {
		return duplicateDescErr
	}

	// Only after all tests have passed, actually register.
	r.collectorsByID[collectorID] = c
	for hash := range newDescIDs {
		r.descIDs[hash] = struct{}{}
	}
	for name, dimHash := range newDimHashesByName {
		r.dimHashesByName[name] = dimHash
	}
	return nil
}

// Unregister implements Registerer.
func (r *Registry) Unregister(c Collector) bool {
	var (
		descChan    = make(chan *Desc, capDescChan)
		descIDs     = map[uint64]struct{}{}
		collectorID uint64 // Just a sum of the desc IDs.
	)
	go func() {
		c.Describe(descChan)
		close(descChan)
	}()
	for desc := range descChan {
		if _, exists := descIDs[desc.id]; !exists {
			collectorID += desc.id
			descIDs[desc.id] = struct{}{}
		}
	}

	r.mtx.RLock()
	if _, exists := r.collectorsByID[collectorID]; !exists {
		r.mtx.RUnlock()
		return false
	}
	r.mtx.RUnlock()

	r.mtx.Lock()
	defer r.mtx.Unlock()

	delete(r.collectorsByID, collectorID)
	for id := range descIDs {
		delete(r.descIDs, id)
	}
	// dimHashesByName is left untouched as those must be consistent
	// throughout the lifetime of a program.
	return true
}

// MustRegister implements Registerer.
func (r *Registry) MustRegister(cs ...Collector) {
	for _, c := range cs {
		if err := r.Register(c); err != nil {
			panic(err)
		}
	}
}

// Deliver implements Deliverer.
func (r *Registry) Deliver() ([]*dto.MetricFamily, error) {
	var (
		metricChan        = make(chan Metric, capMetricChan)
		metricHashes      = map[uint64]struct{}{}
		wg                sync.WaitGroup
		errs              error               // The collected errors to return in the end.
		registeredDescIDs map[uint64]struct{} // Only used for pedantic checks
	)

	r.mtx.RLock()
	metricFamiliesByName := make(map[string]*dto.MetricFamily, len(r.dimHashesByName))

	// Scatter.
	// (Collectors could be complex and slow, so we call them all at once.)
	wg.Add(len(r.collectorsByID))
	go func() {
		wg.Wait()
		close(metricChan)
	}()
	for _, collector := range r.collectorsByID {
		go func(collector Collector) {
			defer wg.Done()
			collector.Collect(metricChan)
		}(collector)
	}

	// In case pedantic checks are enabled, we have to copy the map before
	// giving up the RLock.
	if r.pedanticChecksEnabled {
		registeredDescIDs = make(map[uint64]struct{}, len(r.descIDs))
		for id := range r.descIDs {
			registeredDescIDs[id] = struct{}{}
		}
	}

	r.mtx.RUnlock()

	// Drain metricChan in case of premature return.
	defer func() {
		for _ = range metricChan {
		}
	}()

	// Gather.
	for metric := range metricChan {
		// This could be done concurrently, too, but it required locking
		// of metricFamiliesByName (and of metricHashes if checks are
		// enabled). Most likely not worth it.
		desc := metric.Desc()
		metricFamily, ok := metricFamiliesByName[desc.fqName]
		if !ok {
			metricFamily = &dto.MetricFamily{}
			metricFamily.Name = proto.String(desc.fqName)
			metricFamily.Help = proto.String(desc.help)
			metricFamiliesByName[desc.fqName] = metricFamily
		}
		dtoMetric := &dto.Metric{}
		if err := metric.Write(dtoMetric); err != nil {
			errs = multierror.Append(errs, fmt.Errorf(
				"error collecting metric %v: %s", desc, err,
			))
			continue
		}
		switch {
		case metricFamily.Type != nil:
			// Type already set. We are good.
		case dtoMetric.Gauge != nil:
			metricFamily.Type = dto.MetricType_GAUGE.Enum()
		case dtoMetric.Counter != nil:
			metricFamily.Type = dto.MetricType_COUNTER.Enum()
		case dtoMetric.Summary != nil:
			metricFamily.Type = dto.MetricType_SUMMARY.Enum()
		case dtoMetric.Untyped != nil:
			metricFamily.Type = dto.MetricType_UNTYPED.Enum()
		case dtoMetric.Histogram != nil:
			metricFamily.Type = dto.MetricType_HISTOGRAM.Enum()
		default:
			errs = multierror.Append(errs, fmt.Errorf(
				"empty metric collected: %s", dtoMetric,
			))
			continue
		}
		if err := r.checkConsistency(metricFamily, dtoMetric, desc, metricHashes, registeredDescIDs); err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		metricFamily.Metric = append(metricFamily.Metric, dtoMetric)
	}

	r.mtx.RLock()
	if r.metricFamilyInjectionHook != nil {
		for _, mf := range r.metricFamilyInjectionHook() {
			existingMF, exists := metricFamiliesByName[mf.GetName()]
			if !exists {
				existingMF = &dto.MetricFamily{}
				existingMF.Name = mf.Name
				existingMF.Help = mf.Help
				existingMF.Type = mf.Type
				metricFamiliesByName[mf.GetName()] = existingMF

			}
			for _, m := range mf.Metric {
				if err := r.checkConsistency(existingMF, m, nil, metricHashes, nil); err != nil {
					errs = multierror.Append(errs, err)
					continue
				}
				existingMF.Metric = append(existingMF.Metric, m)
			}
		}
	}
	r.mtx.RUnlock()

	// Now that MetricFamilies are all set, sort their Metrics
	// lexicographically by their label values.
	for _, mf := range metricFamiliesByName {
		sort.Sort(metricSorter(mf.Metric))
	}

	// Write out MetricFamilies sorted by their name, skipping those without
	// metrics.
	names := make([]string, 0, len(metricFamiliesByName))
	for name, mf := range metricFamiliesByName {
		if len(mf.Metric) > 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	result := make([]*dto.MetricFamily, 0, len(names))
	for _, name := range names {
		result = append(result, metricFamiliesByName[name])
	}
	return result, errs
}

func (r *Registry) checkConsistency(
	metricFamily *dto.MetricFamily,
	dtoMetric *dto.Metric,
	desc *Desc,
	metricHashes map[uint64]struct{},
	registeredDescIDs map[uint64]struct{},
) error {

	// Type consistency with metric family.
	if metricFamily.GetType() == dto.MetricType_GAUGE && dtoMetric.Gauge == nil ||
		metricFamily.GetType() == dto.MetricType_COUNTER && dtoMetric.Counter == nil ||
		metricFamily.GetType() == dto.MetricType_SUMMARY && dtoMetric.Summary == nil ||
		metricFamily.GetType() == dto.MetricType_HISTOGRAM && dtoMetric.Histogram == nil ||
		metricFamily.GetType() == dto.MetricType_UNTYPED && dtoMetric.Untyped == nil {
		return fmt.Errorf(
			"collected metric %s %s is not a %s",
			metricFamily.GetName(), dtoMetric, metricFamily.GetType(),
		)
	}

	// Is the metric unique (i.e. no other metric with the same name and the same label values)?
	h := hashNew()
	h = hashAdd(h, metricFamily.GetName())
	h = hashAddByte(h, separatorByte)
	// Make sure label pairs are sorted. We depend on it for the consistency
	// check.
	sort.Sort(LabelPairSorter(dtoMetric.Label))
	for _, lp := range dtoMetric.Label {
		h = hashAdd(h, lp.GetValue())
		h = hashAddByte(h, separatorByte)
	}
	if _, exists := metricHashes[h]; exists {
		return fmt.Errorf(
			"collected metric %s %s was collected before with the same name and label values",
			metricFamily.GetName(), dtoMetric,
		)
	}
	metricHashes[h] = struct{}{}

	if desc == nil || !r.pedanticChecksEnabled {
		return nil // Nothing left to check if we have no desc.
	}

	// Desc help consistency with metric family help.
	if metricFamily.GetHelp() != desc.help {
		return fmt.Errorf(
			"collected metric %s %s has help %q but should have %q",
			metricFamily.GetName(), dtoMetric, metricFamily.GetHelp(), desc.help,
		)
	}

	// Is the desc consistent with the content of the metric?
	lpsFromDesc := make([]*dto.LabelPair, 0, len(dtoMetric.Label))
	lpsFromDesc = append(lpsFromDesc, desc.constLabelPairs...)
	for _, l := range desc.variableLabels {
		lpsFromDesc = append(lpsFromDesc, &dto.LabelPair{
			Name: proto.String(l),
		})
	}
	if len(lpsFromDesc) != len(dtoMetric.Label) {
		return fmt.Errorf(
			"labels in collected metric %s %s are inconsistent with descriptor %s",
			metricFamily.GetName(), dtoMetric, desc,
		)
	}
	sort.Sort(LabelPairSorter(lpsFromDesc))
	for i, lpFromDesc := range lpsFromDesc {
		lpFromMetric := dtoMetric.Label[i]
		if lpFromDesc.GetName() != lpFromMetric.GetName() ||
			lpFromDesc.Value != nil && lpFromDesc.GetValue() != lpFromMetric.GetValue() {
			return fmt.Errorf(
				"labels in collected metric %s %s are inconsistent with descriptor %s",
				metricFamily.GetName(), dtoMetric, desc,
			)
		}
	}

	// Is the desc registered?
	if _, exist := registeredDescIDs[desc.id]; !exist {
		return fmt.Errorf(
			"collected metric %s %s with unregistered descriptor %s",
			metricFamily.GetName(), dtoMetric, desc,
		)
	}

	return nil
}

// SetInjectionHook sets the provided hook to inject MetricFamilies. The hook is
// a function that is called whenever metrics are collected. The MetricFamily
// protobufs returned by the hook function are merged with the metrics collected
// in the usual way.
//
// This is a way to directly inject MetricFamily protobufs managed and owned by
// the caller. The caller has full responsibility. As no registration of the
// injected metrics has happened, there was no check at registration time. If
// the injection results in inconsistent metrics, the Collect call will return
// an error. Some problems may even go undetected, like invalid label names in
// the injected protobufs.
//
// The hook function must be callable at any time and concurrently.
func (r *Registry) SetInjectionHook(hook func() []*dto.MetricFamily) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.metricFamilyInjectionHook = hook
}

// metricSorter is a sortable slice of *dto.Metric.
type metricSorter []*dto.Metric

func (s metricSorter) Len() int {
	return len(s)
}

func (s metricSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s metricSorter) Less(i, j int) bool {
	if len(s[i].Label) != len(s[j].Label) {
		// This should not happen. The metrics are
		// inconsistent. However, we have to deal with the fact, as
		// people might use custom collectors or metric family injection
		// to create inconsistent metrics. So let's simply compare the
		// number of labels in this case. That will still yield
		// reproducible sorting.
		return len(s[i].Label) < len(s[j].Label)
	}
	for n, lp := range s[i].Label {
		vi := lp.GetValue()
		vj := s[j].Label[n].GetValue()
		if vi != vj {
			return vi < vj
		}
	}

	// We should never arrive here. Multiple metrics with the same
	// label set in the same scrape will lead to undefined ingestion
	// behavior. However, as above, we have to provide stable sorting
	// here, even for inconsistent metrics. So sort equal metrics
	// by their timestamp, with missing timestamps (implying "now")
	// coming last.
	if s[i].TimestampMs == nil {
		return false
	}
	if s[j].TimestampMs == nil {
		return true
	}
	return s[i].GetTimestampMs() < s[j].GetTimestampMs()
}
