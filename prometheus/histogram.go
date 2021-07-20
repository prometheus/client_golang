// Copyright 2015 The Prometheus Authors
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
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	//nolint:staticcheck // Ignore SA1019. Need to keep deprecated package for compatibility.
	"github.com/golang/protobuf/proto"

	dto "github.com/prometheus/client_model/go"
)

// sparseBounds for the frac of observed values. Only relevant for schema > 0.
// Position in the slice is the schema. (0 is never used, just here for
// convenience of using the schema directly as the index.)
//
// TODO(beorn7): Currently, we do a binary search into these slices. There are
// ways to turn it into a small number of simple array lookups. It probably only
// matters for schema 5 and beyond, but should be investigated. See this comment
// as a starting point:
// https://github.com/open-telemetry/opentelemetry-specification/issues/1776#issuecomment-870164310
var sparseBounds = [][]float64{
	// Schema "0":
	[]float64{0.5},
	// Schema 1:
	[]float64{0.5, 0.7071067811865475},
	// Schema 2:
	[]float64{0.5, 0.5946035575013605, 0.7071067811865475, 0.8408964152537144},
	// Schema 3:
	[]float64{0.5, 0.5452538663326288, 0.5946035575013605, 0.6484197773255048,
		0.7071067811865475, 0.7711054127039704, 0.8408964152537144, 0.9170040432046711},
	// Schema 4:
	[]float64{0.5, 0.5221368912137069, 0.5452538663326288, 0.5693943173783458,
		0.5946035575013605, 0.620928906036742, 0.6484197773255048, 0.6771277734684463,
		0.7071067811865475, 0.7384130729697496, 0.7711054127039704, 0.805245165974627,
		0.8408964152537144, 0.8781260801866495, 0.9170040432046711, 0.9576032806985735},
	// Schema 5:
	[]float64{0.5, 0.5109485743270583, 0.5221368912137069, 0.5335702003384117,
		0.5452538663326288, 0.5571933712979462, 0.5693943173783458, 0.5818624293887887,
		0.5946035575013605, 0.6076236799902344, 0.620928906036742, 0.6345254785958666,
		0.6484197773255048, 0.6626183215798706, 0.6771277734684463, 0.6919549409819159,
		0.7071067811865475, 0.7225904034885232, 0.7384130729697496, 0.7545822137967112,
		0.7711054127039704, 0.7879904225539431, 0.805245165974627, 0.8228777390769823,
		0.8408964152537144, 0.8593096490612387, 0.8781260801866495, 0.8973545375015533,
		0.9170040432046711, 0.9370838170551498, 0.9576032806985735, 0.9785720620876999},
	// Schema 6:
	[]float64{0.5, 0.5054446430258502, 0.5109485743270583, 0.5165124395106142,
		0.5221368912137069, 0.5278225891802786, 0.5335702003384117, 0.5393803988785598,
		0.5452538663326288, 0.5511912916539204, 0.5571933712979462, 0.5632608093041209,
		0.5693943173783458, 0.5755946149764913, 0.5818624293887887, 0.5881984958251406,
		0.5946035575013605, 0.6010783657263515, 0.6076236799902344, 0.6142402680534349,
		0.620928906036742, 0.6276903785123455, 0.6345254785958666, 0.6414350080393891,
		0.6484197773255048, 0.6554806057623822, 0.6626183215798706, 0.6698337620266515,
		0.6771277734684463, 0.6845012114872953, 0.6919549409819159, 0.6994898362691555,
		0.7071067811865475, 0.7148066691959849, 0.7225904034885232, 0.7304588970903234,
		0.7384130729697496, 0.7464538641456323, 0.7545822137967112, 0.762799075372269,
		0.7711054127039704, 0.7795022001189185, 0.7879904225539431, 0.7965710756711334,
		0.805245165974627, 0.8140137109286738, 0.8228777390769823, 0.8318382901633681,
		0.8408964152537144, 0.8500531768592616, 0.8593096490612387, 0.8686669176368529,
		0.8781260801866495, 0.8876882462632604, 0.8973545375015533, 0.9071260877501991,
		0.9170040432046711, 0.9269895625416926, 0.9370838170551498, 0.9472879907934827,
		0.9576032806985735, 0.9680308967461471, 0.9785720620876999, 0.9892280131939752},
	// Schema 7:
	[]float64{0.5, 0.5027149505564014, 0.5054446430258502, 0.5081891574554764,
		0.5109485743270583, 0.5137229745593818, 0.5165124395106142, 0.5193170509806894,
		0.5221368912137069, 0.5249720429003435, 0.5278225891802786, 0.5306886136446309,
		0.5335702003384117, 0.5364674337629877, 0.5393803988785598, 0.5423091811066545,
		0.5452538663326288, 0.5482145409081883, 0.5511912916539204, 0.5541842058618393,
		0.5571933712979462, 0.5602188762048033, 0.5632608093041209, 0.5663192597993595,
		0.5693943173783458, 0.572486072215902, 0.5755946149764913, 0.5787200368168754,
		0.5818624293887887, 0.585021884841625, 0.5881984958251406, 0.5913923554921704,
		0.5946035575013605, 0.5978321960199137, 0.6010783657263515, 0.6043421618132907,
		0.6076236799902344, 0.6109230164863786, 0.6142402680534349, 0.6175755319684665,
		0.620928906036742, 0.6243004885946023, 0.6276903785123455, 0.6310986751971253,
		0.6345254785958666, 0.637970889198196, 0.6414350080393891, 0.6449179367033329,
		0.6484197773255048, 0.6519406325959679, 0.6554806057623822, 0.659039800633032,
		0.6626183215798706, 0.6662162735415805, 0.6698337620266515, 0.6734708931164728,
		0.6771277734684463, 0.6808045103191123, 0.6845012114872953, 0.688217985377265,
		0.6919549409819159, 0.6957121878859629, 0.6994898362691555, 0.7032879969095076,
		0.7071067811865475, 0.7109463010845827, 0.7148066691959849, 0.718687998724491,
		0.7225904034885232, 0.7265139979245261, 0.7304588970903234, 0.7344252166684908,
		0.7384130729697496, 0.7424225829363761, 0.7464538641456323, 0.7505070348132126,
		0.7545822137967112, 0.7586795205991071, 0.762799075372269, 0.7669409989204777,
		0.7711054127039704, 0.7752924388424999, 0.7795022001189185, 0.7837348199827764,
		0.7879904225539431, 0.7922691326262467, 0.7965710756711334, 0.8008963778413465,
		0.805245165974627, 0.8096175675974316, 0.8140137109286738, 0.8184337248834821,
		0.8228777390769823, 0.8273458838280969, 0.8318382901633681, 0.8363550898207981,
		0.8408964152537144, 0.8454623996346523, 0.8500531768592616, 0.8546688815502312,
		0.8593096490612387, 0.8639756154809185, 0.8686669176368529, 0.8733836930995842,
		0.8781260801866495, 0.8828942179666361, 0.8876882462632604, 0.8925083056594671,
		0.8973545375015533, 0.9022270839033115, 0.9071260877501991, 0.9120516927035263,
		0.9170040432046711, 0.9219832844793128, 0.9269895625416926, 0.9320230241988943,
		0.9370838170551498, 0.9421720895161669, 0.9472879907934827, 0.9524316709088368,
		0.9576032806985735, 0.9628029718180622, 0.9680308967461471, 0.9732872087896164,
		0.9785720620876999, 0.9838856116165875, 0.9892280131939752, 0.9945994234836328},
	// Schema 8:
	[]float64{0.5, 0.5013556375251013, 0.5027149505564014, 0.5040779490592088,
		0.5054446430258502, 0.5068150424757447, 0.5081891574554764, 0.509566998038869,
		0.5109485743270583, 0.5123338964485679, 0.5137229745593818, 0.5151158188430205,
		0.5165124395106142, 0.5179128468009786, 0.5193170509806894, 0.520725062344158,
		0.5221368912137069, 0.5235525479396449, 0.5249720429003435, 0.526395386502313,
		0.5278225891802786, 0.5292536613972564, 0.5306886136446309, 0.5321274564422321,
		0.5335702003384117, 0.5350168559101208, 0.5364674337629877, 0.5379219445313954,
		0.5393803988785598, 0.5408428074966075, 0.5423091811066545, 0.5437795304588847,
		0.5452538663326288, 0.5467321995364429, 0.5482145409081883, 0.549700901315111,
		0.5511912916539204, 0.5526857228508706, 0.5541842058618393, 0.5556867516724088,
		0.5571933712979462, 0.5587040757836845, 0.5602188762048033, 0.5617377836665098,
		0.5632608093041209, 0.564787964283144, 0.5663192597993595, 0.5678547070789026,
		0.5693943173783458, 0.5709381019847808, 0.572486072215902, 0.5740382394200894,
		0.5755946149764913, 0.5771552102951081, 0.5787200368168754, 0.5802891060137493,
		0.5818624293887887, 0.5834400184762408, 0.585021884841625, 0.5866080400818185,
		0.5881984958251406, 0.5897932637314379, 0.5913923554921704, 0.5929957828304968,
		0.5946035575013605, 0.5962156912915756, 0.5978321960199137, 0.5994530835371903,
		0.6010783657263515, 0.6027080545025619, 0.6043421618132907, 0.6059806996384005,
		0.6076236799902344, 0.6092711149137041, 0.6109230164863786, 0.6125793968185725,
		0.6142402680534349, 0.6159056423670379, 0.6175755319684665, 0.6192499490999082,
		0.620928906036742, 0.622612415087629, 0.6243004885946023, 0.6259931389331581,
		0.6276903785123455, 0.6293922197748583, 0.6310986751971253, 0.6328097572894031,
		0.6345254785958666, 0.6362458516947014, 0.637970889198196, 0.6397006037528346,
		0.6414350080393891, 0.6431741147730128, 0.6449179367033329, 0.6466664866145447,
		0.6484197773255048, 0.6501778216898253, 0.6519406325959679, 0.6537082229673385,
		0.6554806057623822, 0.6572577939746774, 0.659039800633032, 0.6608266388015788,
		0.6626183215798706, 0.6644148621029772, 0.6662162735415805, 0.6680225691020727,
		0.6698337620266515, 0.6716498655934177, 0.6734708931164728, 0.6752968579460171,
		0.6771277734684463, 0.6789636531064505, 0.6808045103191123, 0.6826503586020058,
		0.6845012114872953, 0.6863570825438342, 0.688217985377265, 0.690083933630119,
		0.6919549409819159, 0.6938310211492645, 0.6957121878859629, 0.6975984549830999,
		0.6994898362691555, 0.7013863456101023, 0.7032879969095076, 0.7051948041086352,
		0.7071067811865475, 0.7090239421602076, 0.7109463010845827, 0.7128738720527471,
		0.7148066691959849, 0.7167447066838943, 0.718687998724491, 0.7206365595643126,
		0.7225904034885232, 0.7245495448210174, 0.7265139979245261, 0.7284837772007218,
		0.7304588970903234, 0.7324393720732029, 0.7344252166684908, 0.7364164454346837,
		0.7384130729697496, 0.7404151139112358, 0.7424225829363761, 0.7444354947621984,
		0.7464538641456323, 0.7484777058836176, 0.7505070348132126, 0.7525418658117031,
		0.7545822137967112, 0.7566280937263048, 0.7586795205991071, 0.7607365094544071,
		0.762799075372269, 0.7648672334736434, 0.7669409989204777, 0.7690203869158282,
		0.7711054127039704, 0.7731960915705107, 0.7752924388424999, 0.7773944698885442,
		0.7795022001189185, 0.7816156449856788, 0.7837348199827764, 0.7858597406461707,
		0.7879904225539431, 0.7901268813264122, 0.7922691326262467, 0.7944171921585818,
		0.7965710756711334, 0.7987307989543135, 0.8008963778413465, 0.8030678282083853,
		0.805245165974627, 0.8074284071024302, 0.8096175675974316, 0.8118126635086642,
		0.8140137109286738, 0.8162207259936375, 0.8184337248834821, 0.820652723822003,
		0.8228777390769823, 0.8251087869603088, 0.8273458838280969, 0.8295890460808079,
		0.8318382901633681, 0.8340936325652911, 0.8363550898207981, 0.8386226785089391,
		0.8408964152537144, 0.8431763167241966, 0.8454623996346523, 0.8477546807446661,
		0.8500531768592616, 0.8523579048290255, 0.8546688815502312, 0.8569861239649629,
		0.8593096490612387, 0.8616394738731368, 0.8639756154809185, 0.8663180910111553,
		0.8686669176368529, 0.871022112577578, 0.8733836930995842, 0.8757516765159389,
		0.8781260801866495, 0.8805069215187917, 0.8828942179666361, 0.8852879870317771,
		0.8876882462632604, 0.890095013257712, 0.8925083056594671, 0.8949281411607002,
		0.8973545375015533, 0.8997875124702672, 0.9022270839033115, 0.9046732696855155,
		0.9071260877501991, 0.909585556079304, 0.9120516927035263, 0.9145245157024483,
		0.9170040432046711, 0.9194902933879467, 0.9219832844793128, 0.9244830347552253,
		0.9269895625416926, 0.92950288621441, 0.9320230241988943, 0.9345499949706191,
		0.9370838170551498, 0.93962450902828, 0.9421720895161669, 0.9447265771954693,
		0.9472879907934827, 0.9498563490882775, 0.9524316709088368, 0.9550139751351947,
		0.9576032806985735, 0.9601996065815236, 0.9628029718180622, 0.9654133954938133,
		0.9680308967461471, 0.9706554947643201, 0.9732872087896164, 0.9759260581154889,
		0.9785720620876999, 0.9812252401044634, 0.9838856116165875, 0.9865531961276168,
		0.9892280131939752, 0.9919100824251095, 0.9945994234836328, 0.9972960560854698},
}

// The sparseBounds above can be generated with the code below.
// TODO(beorn7): Actually do it via go generate.
//
// var sparseBounds [][]float64 = make([][]float64, 9)
//
// func init() {
// 	// Populate sparseBounds.
// 	numBuckets := 1
// 	for i := range sparseBounds {
// 		bounds := []float64{0.5}
// 		factor := math.Exp2(math.Exp2(float64(-i)))
// 		for j := 0; j < numBuckets-1; j++ {
// 			var bound float64
// 			if (j+1)%2 == 0 {
// 				// Use previously calculated value for increased precision.
// 				bound = sparseBounds[i-1][j/2+1]
// 			} else {
// 				bound = bounds[j] * factor
// 			}
// 			bounds = append(bounds, bound)
// 		}
// 		numBuckets *= 2
// 		sparseBounds[i] = bounds
// 	}
// }

// A Histogram counts individual observations from an event or sample stream in
// configurable buckets. Similar to a summary, it also provides a sum of
// observations and an observation count.
//
// On the Prometheus server, quantiles can be calculated from a Histogram using
// the histogram_quantile function in the query language.
//
// Note that Histograms, in contrast to Summaries, can be aggregated with the
// Prometheus query language (see the documentation for detailed
// procedures). However, Histograms require the user to pre-define suitable
// buckets, and they are in general less accurate. The Observe method of a
// Histogram has a very low performance overhead in comparison with the Observe
// method of a Summary.
//
// To create Histogram instances, use NewHistogram.
type Histogram interface {
	Metric
	Collector

	// Observe adds a single observation to the histogram. Observations are
	// usually positive or zero. Negative observations are accepted but
	// prevent current versions of Prometheus from properly detecting
	// counter resets in the sum of observations. See
	// https://prometheus.io/docs/practices/histograms/#count-and-sum-of-observations
	// for details.
	Observe(float64)
}

// bucketLabel is used for the label that defines the upper bound of a
// bucket of a histogram ("le" -> "less or equal").
const bucketLabel = "le"

// DefBuckets are the default Histogram buckets. The default buckets are
// tailored to broadly measure the response time (in seconds) of a network
// service. Most likely, however, you will be required to define buckets
// customized to your use case.
var DefBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

// DefSparseBucketsZeroThreshold is the default value for
// SparseBucketsZeroThreshold in the HistogramOpts.
//
// The value is 2^-128 (or 0.5*2^-127 in the actual IEEE 754 representation),
// which is a bucket boundary at all possible resolutions.
const DefSparseBucketsZeroThreshold = 2.938735877055719e-39

var errBucketLabelNotAllowed = fmt.Errorf(
	"%q is not allowed as label name in histograms", bucketLabel,
)

// LinearBuckets creates 'count' buckets, each 'width' wide, where the lowest
// bucket has an upper bound of 'start'. The final +Inf bucket is not counted
// and not included in the returned slice. The returned slice is meant to be
// used for the Buckets field of HistogramOpts.
//
// The function panics if 'count' is zero or negative.
func LinearBuckets(start, width float64, count int) []float64 {
	if count < 1 {
		panic("LinearBuckets needs a positive count")
	}
	buckets := make([]float64, count)
	for i := range buckets {
		buckets[i] = start
		start += width
	}
	return buckets
}

// ExponentialBuckets creates 'count' buckets, where the lowest bucket has an
// upper bound of 'start' and each following bucket's upper bound is 'factor'
// times the previous bucket's upper bound. The final +Inf bucket is not counted
// and not included in the returned slice. The returned slice is meant to be
// used for the Buckets field of HistogramOpts.
//
// The function panics if 'count' is 0 or negative, if 'start' is 0 or negative,
// or if 'factor' is less than or equal 1.
func ExponentialBuckets(start, factor float64, count int) []float64 {
	if count < 1 {
		panic("ExponentialBuckets needs a positive count")
	}
	if start <= 0 {
		panic("ExponentialBuckets needs a positive start value")
	}
	if factor <= 1 {
		panic("ExponentialBuckets needs a factor greater than 1")
	}
	buckets := make([]float64, count)
	for i := range buckets {
		buckets[i] = start
		start *= factor
	}
	return buckets
}

// HistogramOpts bundles the options for creating a Histogram metric. It is
// mandatory to set Name to a non-empty string. All other fields are optional
// and can safely be left at their zero value, although it is strongly
// encouraged to set a Help string.
type HistogramOpts struct {
	// Namespace, Subsystem, and Name are components of the fully-qualified
	// name of the Histogram (created by joining these components with
	// "_"). Only Name is mandatory, the others merely help structuring the
	// name. Note that the fully-qualified name of the Histogram must be a
	// valid Prometheus metric name.
	Namespace string
	Subsystem string
	Name      string

	// Help provides information about this Histogram.
	//
	// Metrics with the same fully-qualified name must have the same Help
	// string.
	Help string

	// ConstLabels are used to attach fixed labels to this metric. Metrics
	// with the same fully-qualified name must have the same label names in
	// their ConstLabels.
	//
	// ConstLabels are only used rarely. In particular, do not use them to
	// attach the same labels to all your metrics. Those use cases are
	// better covered by target labels set by the scraping Prometheus
	// server, or by one specific metric (e.g. a build_info or a
	// machine_role metric). See also
	// https://prometheus.io/docs/instrumenting/writing_exporters/#target-labels-not-static-scraped-labels
	ConstLabels Labels

	// Buckets defines the buckets into which observations are counted. Each
	// element in the slice is the upper inclusive bound of a bucket. The
	// values must be sorted in strictly increasing order. There is no need
	// to add a highest bucket with +Inf bound, it will be added
	// implicitly. If Buckets is left as nil or set to a slice of length
	// zero, it is replaced by default buckets. The default buckets are
	// DefBuckets if no sparse buckets (see below) are used, otherwise the
	// default is no buckets. (In other words, if you want to use both
	// reguler buckets and sparse buckets, you have to define the regular
	// buckets here explicitly.)
	Buckets []float64

	// If SparseBucketsFactor is greater than one, sparse buckets are used
	// (in addition to the regular buckets, if defined above). Sparse
	// buckets are exponential buckets covering the whole float64 range
	// (with the exception of the “zero” bucket, see
	// SparseBucketsZeroThreshold below). From any one bucket to the next,
	// the width of the bucket grows by a constant factor.
	// SparseBucketsFactor provides an upper bound for this factor
	// (exception see below). The smaller SparseBucketsFactor, the more
	// buckets will be used and thus the more costly the histogram will
	// become. A generally good trade-off between cost and accuracy is a
	// value of 1.1 (each bucket is at most 10% wider than the previous
	// one), which will result in each power of two divided into 8 buckets
	// (e.g. there will be 8 buckets between 1 and 2, same as between 2 and
	// 4, and 4 and 8, etc.).
	//
	// Details about the actually used factor: The factor is calculated as
	// 2^(2^n), where n is an integer number between (and including) -8 and
	// 4. n is chosen so that the resulting factor is the largest that is
	// still smaller or equal to SparseBucketsFactor. Note that the smallest
	// possible factor is therefore approx. 1.00271 (i.e. 2^(2^-8) ). If
	// SparseBucketsFactor is greater than 1 but smaller than 2^(2^-8), then
	// the actually used factor is still 2^(2^-8) even though it is larger
	// than the provided SparseBucketsFactor.
	SparseBucketsFactor float64
	// All observations with an absolute value of less or equal
	// SparseBucketsZeroThreshold are accumulated into a “zero” bucket. For
	// best results, this should be close to a bucket boundary. This is
	// usually the case if picking a power of two. If
	// SparseBucketsZeroThreshold is left at zero,
	// DefSparseBucketsZeroThreshold is used as the threshold. If it is set
	// to a negative value, a threshold of zero is used, i.e. only
	// observations of precisely zero will go into the zero
	// bucket. (TODO(beorn7): That's obviously weird and just a consequence
	// of making the zero value of HistogramOpts meaningful. Has to be
	// solved more elegantly in the final version.)
	SparseBucketsZeroThreshold float64
	// TODO(beorn7): Need a setting to limit total bucket count and to
	// configure a strategy to enforce the limit, e.g. if minimum duration
	// after last reset, reset. If not, half the resolution and/or expand
	// the zero bucket.
}

// NewHistogram creates a new Histogram based on the provided HistogramOpts. It
// panics if the buckets in HistogramOpts are not in strictly increasing order.
//
// The returned implementation also implements ExemplarObserver. It is safe to
// perform the corresponding type assertion. Exemplars are tracked separately
// for each bucket.
func NewHistogram(opts HistogramOpts) Histogram {
	return newHistogram(
		NewDesc(
			BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
			opts.Help,
			nil,
			opts.ConstLabels,
		),
		opts,
	)
}

func newHistogram(desc *Desc, opts HistogramOpts, labelValues ...string) Histogram {
	if len(desc.variableLabels) != len(labelValues) {
		panic(makeInconsistentCardinalityError(desc.fqName, desc.variableLabels, labelValues))
	}

	for _, n := range desc.variableLabels {
		if n == bucketLabel {
			panic(errBucketLabelNotAllowed)
		}
	}
	for _, lp := range desc.constLabelPairs {
		if lp.GetName() == bucketLabel {
			panic(errBucketLabelNotAllowed)
		}
	}

	h := &histogram{
		desc:        desc,
		upperBounds: opts.Buckets,
		labelPairs:  MakeLabelPairs(desc, labelValues),
		counts:      [2]*histogramCounts{{}, {}},
		now:         time.Now,
	}
	if len(h.upperBounds) == 0 && opts.SparseBucketsFactor <= 1 {
		h.upperBounds = DefBuckets
	}
	if opts.SparseBucketsFactor <= 1 {
		h.sparseSchema = math.MinInt32 // To mark that there are no sparse buckets.
	} else {
		switch {
		case opts.SparseBucketsZeroThreshold > 0:
			h.sparseThreshold = opts.SparseBucketsZeroThreshold
		case opts.SparseBucketsZeroThreshold == 0:
			h.sparseThreshold = DefSparseBucketsZeroThreshold
		} // Leave h.sparseThreshold at 0 otherwise.
		h.sparseSchema = pickSparseSchema(opts.SparseBucketsFactor)
	}
	for i, upperBound := range h.upperBounds {
		if i < len(h.upperBounds)-1 {
			if upperBound >= h.upperBounds[i+1] {
				panic(fmt.Errorf(
					"histogram buckets must be in increasing order: %f >= %f",
					upperBound, h.upperBounds[i+1],
				))
			}
		} else {
			if math.IsInf(upperBound, +1) {
				// The +Inf bucket is implicit. Remove it here.
				h.upperBounds = h.upperBounds[:i]
			}
		}
	}
	// Finally we know the final length of h.upperBounds and can make buckets
	// for both counts as well as exemplars:
	h.counts[0].buckets = make([]uint64, len(h.upperBounds))
	h.counts[1].buckets = make([]uint64, len(h.upperBounds))
	h.exemplars = make([]atomic.Value, len(h.upperBounds)+1)

	h.init(h) // Init self-collection.
	return h
}

type histogramCounts struct {
	// sumBits contains the bits of the float64 representing the sum of all
	// observations. sumBits and count have to go first in the struct to
	// guarantee alignment for atomic operations.
	// http://golang.org/pkg/sync/atomic/#pkg-note-BUG
	sumBits uint64
	count   uint64
	buckets []uint64
	// sparse buckets are implemented with a sync.Map for now. A dedicated
	// data structure will likely be more efficient. There are separate maps
	// for negative and positive observations. The map's value is an *int64,
	// counting observations in that bucket. (Note that we don't use uint64
	// as an int64 won't overflow in practice, and working with signed
	// numbers from the beginning simplifies the handling of deltas.) The
	// map's key is the index of the bucket according to the used
	// sparseSchema. Index 0 is for an upper bound of 1.
	sparseBucketsPositive, sparseBucketsNegative sync.Map
	// sparseZeroBucket counts all (positive and negative) observations in
	// the zero bucket (with an absolute value less or equal
	// SparseBucketsZeroThreshold).
	sparseZeroBucket uint64
}

// observe manages the parts of observe that only affects
// histogramCounts. doSparse is true if spare buckets should be done,
// too. whichSparse is 0 for the sparseZeroBucket and +1 or -1 for
// sparseBucketsPositive or sparseBucketsNegative, respectively. sparseKey is
// the key of the sparse bucket to use.
func (hc *histogramCounts) observe(v float64, bucket int, doSparse bool, whichSparse int, sparseKey int) {
	if bucket < len(hc.buckets) {
		atomic.AddUint64(&hc.buckets[bucket], 1)
	}
	for {
		oldBits := atomic.LoadUint64(&hc.sumBits)
		newBits := math.Float64bits(math.Float64frombits(oldBits) + v)
		if atomic.CompareAndSwapUint64(&hc.sumBits, oldBits, newBits) {
			break
		}
	}
	if doSparse {
		switch whichSparse {
		case 0:
			atomic.AddUint64(&hc.sparseZeroBucket, 1)
		case +1:
			addToSparseBucket(&hc.sparseBucketsPositive, sparseKey, 1)
		case -1:
			addToSparseBucket(&hc.sparseBucketsNegative, sparseKey, 1)
		default:
			panic(fmt.Errorf("invalid value for whichSparse: %d", whichSparse))
		}
	}
	// Increment count last as we take it as a signal that the observation
	// is complete.
	atomic.AddUint64(&hc.count, 1)
}

func addToSparseBucket(buckets *sync.Map, key int, increment int64) {
	if existingBucket, ok := buckets.Load(key); ok {
		// Fast path without allocation.
		atomic.AddInt64(existingBucket.(*int64), increment)
		return
	}
	// Bucket doesn't exist yet. Slow path allocating new counter.
	newBucket := increment // TODO(beorn7): Check if this is sufficient to not let increment escape.
	if actualBucket, loaded := buckets.LoadOrStore(key, &newBucket); loaded {
		// The bucket was created concurrently in another goroutine.
		// Have to increment after all.
		atomic.AddInt64(actualBucket.(*int64), increment)
	}
}

type histogram struct {
	// countAndHotIdx enables lock-free writes with use of atomic updates.
	// The most significant bit is the hot index [0 or 1] of the count field
	// below. Observe calls update the hot one. All remaining bits count the
	// number of Observe calls. Observe starts by incrementing this counter,
	// and finish by incrementing the count field in the respective
	// histogramCounts, as a marker for completion.
	//
	// Calls of the Write method (which are non-mutating reads from the
	// perspective of the histogram) swap the hot–cold under the writeMtx
	// lock. A cooldown is awaited (while locked) by comparing the number of
	// observations with the initiation count. Once they match, then the
	// last observation on the now cool one has completed. All cold fields must
	// be merged into the new hot before releasing writeMtx.
	//
	// Fields with atomic access first! See alignment constraint:
	// http://golang.org/pkg/sync/atomic/#pkg-note-BUG
	countAndHotIdx uint64

	selfCollector
	desc     *Desc
	writeMtx sync.Mutex // Only used in the Write method.

	// Two counts, one is "hot" for lock-free observations, the other is
	// "cold" for writing out a dto.Metric. It has to be an array of
	// pointers to guarantee 64bit alignment of the histogramCounts, see
	// http://golang.org/pkg/sync/atomic/#pkg-note-BUG.
	counts [2]*histogramCounts

	upperBounds     []float64
	labelPairs      []*dto.LabelPair
	exemplars       []atomic.Value // One more than buckets (to include +Inf), each a *dto.Exemplar.
	sparseSchema    int32          // Set to math.MinInt32 if no sparse buckets are used.
	sparseThreshold float64

	now func() time.Time // To mock out time.Now() for testing.
}

func (h *histogram) Desc() *Desc {
	return h.desc
}

func (h *histogram) Observe(v float64) {
	h.observe(v, h.findBucket(v))
}

func (h *histogram) ObserveWithExemplar(v float64, e Labels) {
	i := h.findBucket(v)
	h.observe(v, i)
	h.updateExemplar(v, i, e)
}

func (h *histogram) Write(out *dto.Metric) error {
	// For simplicity, we protect this whole method by a mutex. It is not in
	// the hot path, i.e. Observe is called much more often than Write. The
	// complication of making Write lock-free isn't worth it, if possible at
	// all.
	h.writeMtx.Lock()
	defer h.writeMtx.Unlock()

	// Adding 1<<63 switches the hot index (from 0 to 1 or from 1 to 0)
	// without touching the count bits. See the struct comments for a full
	// description of the algorithm.
	n := atomic.AddUint64(&h.countAndHotIdx, 1<<63)
	// count is contained unchanged in the lower 63 bits.
	count := n & ((1 << 63) - 1)
	// The most significant bit tells us which counts is hot. The complement
	// is thus the cold one.
	hotCounts := h.counts[n>>63]
	coldCounts := h.counts[(^n)>>63]

	// Await cooldown.
	for count != atomic.LoadUint64(&coldCounts.count) {
		runtime.Gosched() // Let observations get work done.
	}

	his := &dto.Histogram{
		Bucket:      make([]*dto.Bucket, len(h.upperBounds)),
		SampleCount: proto.Uint64(count),
		SampleSum:   proto.Float64(math.Float64frombits(atomic.LoadUint64(&coldCounts.sumBits))),
	}
	out.Histogram = his
	out.Label = h.labelPairs

	var cumCount uint64
	for i, upperBound := range h.upperBounds {
		cumCount += atomic.LoadUint64(&coldCounts.buckets[i])
		his.Bucket[i] = &dto.Bucket{
			CumulativeCount: proto.Uint64(cumCount),
			UpperBound:      proto.Float64(upperBound),
		}
		if e := h.exemplars[i].Load(); e != nil {
			his.Bucket[i].Exemplar = e.(*dto.Exemplar)
		}
	}
	// If there is an exemplar for the +Inf bucket, we have to add that bucket explicitly.
	if e := h.exemplars[len(h.upperBounds)].Load(); e != nil {
		b := &dto.Bucket{
			CumulativeCount: proto.Uint64(count),
			UpperBound:      proto.Float64(math.Inf(1)),
			Exemplar:        e.(*dto.Exemplar),
		}
		his.Bucket = append(his.Bucket, b)
	}
	// Add all the cold counts to the new hot counts and reset the cold counts.
	atomic.AddUint64(&hotCounts.count, count)
	atomic.StoreUint64(&coldCounts.count, 0)
	for {
		oldBits := atomic.LoadUint64(&hotCounts.sumBits)
		newBits := math.Float64bits(math.Float64frombits(oldBits) + his.GetSampleSum())
		if atomic.CompareAndSwapUint64(&hotCounts.sumBits, oldBits, newBits) {
			atomic.StoreUint64(&coldCounts.sumBits, 0)
			break
		}
	}
	for i := range h.upperBounds {
		atomic.AddUint64(&hotCounts.buckets[i], atomic.LoadUint64(&coldCounts.buckets[i]))
		atomic.StoreUint64(&coldCounts.buckets[i], 0)
	}
	if h.sparseSchema > math.MinInt32 {
		his.SbZeroThreshold = &h.sparseThreshold
		his.SbSchema = &h.sparseSchema
		zeroBucket := atomic.LoadUint64(&coldCounts.sparseZeroBucket)

		defer func() {
			atomic.AddUint64(&hotCounts.sparseZeroBucket, zeroBucket)
			atomic.StoreUint64(&coldCounts.sparseZeroBucket, 0)
			coldCounts.sparseBucketsPositive.Range(addAndReset(&hotCounts.sparseBucketsPositive))
			coldCounts.sparseBucketsNegative.Range(addAndReset(&hotCounts.sparseBucketsNegative))
		}()

		his.SbZeroCount = proto.Uint64(zeroBucket)
		his.SbNegative = makeSparseBuckets(&coldCounts.sparseBucketsNegative)
		his.SbPositive = makeSparseBuckets(&coldCounts.sparseBucketsPositive)
	}
	return nil
}

func makeSparseBuckets(buckets *sync.Map) *dto.SparseBuckets {
	var ii []int
	buckets.Range(func(k, v interface{}) bool {
		ii = append(ii, k.(int))
		return true
	})
	sort.Ints(ii)

	if len(ii) == 0 {
		return nil
	}

	sbs := dto.SparseBuckets{}
	var prevCount int64
	var nextI int

	appendDelta := func(count int64) {
		*sbs.Span[len(sbs.Span)-1].Length++
		sbs.Delta = append(sbs.Delta, count-prevCount)
		prevCount = count
	}

	for n, i := range ii {
		v, _ := buckets.Load(i)
		count := atomic.LoadInt64(v.(*int64))
		// Multiple spans with only small gaps in between are probably
		// encoded more efficiently as one larger span with a few empty
		// buckets. Needs some research to find the sweet spot. For now,
		// we assume that gaps of one ore two buckets should not create
		// a new span.
		iDelta := int32(i - nextI)
		if n == 0 || iDelta > 2 {
			// We have to create a new span, either because we are
			// at the very beginning, or because we have found a gap
			// of more than two buckets.
			sbs.Span = append(sbs.Span, &dto.SparseBuckets_Span{
				Offset: proto.Int32(iDelta),
				Length: proto.Uint32(0),
			})
		} else {
			// We have found a small gap (or no gap at all).
			// Insert empty buckets as needed.
			for j := int32(0); j < iDelta; j++ {
				appendDelta(0)
			}
		}
		appendDelta(count)
		nextI = i + 1
	}
	return &sbs
}

// addAndReset returns a function to be used with sync.Map.Range of spare
// buckets in coldCounts. It increments the buckets in the provided hotBuckets
// according to the buckets ranged through. It then resets all buckets ranged
// through to 0 (but leaves them in place so that they don't need to get
// recreated on the next scrape).
func addAndReset(hotBuckets *sync.Map) func(k, v interface{}) bool {
	return func(k, v interface{}) bool {
		bucket := v.(*int64)
		addToSparseBucket(hotBuckets, k.(int), atomic.LoadInt64(bucket))
		atomic.StoreInt64(bucket, 0)
		return true
	}
}

// findBucket returns the index of the bucket for the provided value, or
// len(h.upperBounds) for the +Inf bucket.
func (h *histogram) findBucket(v float64) int {
	// TODO(beorn7): For small numbers of buckets (<30), a linear search is
	// slightly faster than the binary search. If we really care, we could
	// switch from one search strategy to the other depending on the number
	// of buckets.
	//
	// Microbenchmarks (BenchmarkHistogramNoLabels):
	// 11 buckets: 38.3 ns/op linear - binary 48.7 ns/op
	// 100 buckets: 78.1 ns/op linear - binary 54.9 ns/op
	// 300 buckets: 154 ns/op linear - binary 61.6 ns/op
	return sort.SearchFloat64s(h.upperBounds, v)
}

// observe is the implementation for Observe without the findBucket part.
func (h *histogram) observe(v float64, bucket int) {
	// Do not add to sparse buckets for NaN observations.
	doSparse := h.sparseSchema > math.MinInt32 && !math.IsNaN(v)
	var whichSparse, sparseKey int
	if doSparse {
		switch {
		case v > h.sparseThreshold:
			whichSparse = +1
		case v < -h.sparseThreshold:
			whichSparse = -1
		}
		frac, exp := math.Frexp(math.Abs(v))
		switch {
		case math.IsInf(v, 0):
			sparseKey = math.MaxInt32 // Largest possible sparseKey.
		case h.sparseSchema > 0:
			bounds := sparseBounds[h.sparseSchema]
			sparseKey = sort.SearchFloat64s(bounds, frac) + (exp-1)*len(bounds)
		default:
			sparseKey = exp
			if frac == 0.5 {
				sparseKey--
			}
			sparseKey /= 1 << -h.sparseSchema
		}
	}
	// We increment h.countAndHotIdx so that the counter in the lower
	// 63 bits gets incremented. At the same time, we get the new value
	// back, which we can use to find the currently-hot counts.
	n := atomic.AddUint64(&h.countAndHotIdx, 1)
	h.counts[n>>63].observe(v, bucket, doSparse, whichSparse, sparseKey)
}

// updateExemplar replaces the exemplar for the provided bucket. With empty
// labels, it's a no-op. It panics if any of the labels is invalid.
func (h *histogram) updateExemplar(v float64, bucket int, l Labels) {
	if l == nil {
		return
	}
	e, err := newExemplar(v, h.now(), l)
	if err != nil {
		panic(err)
	}
	h.exemplars[bucket].Store(e)
}

// HistogramVec is a Collector that bundles a set of Histograms that all share the
// same Desc, but have different values for their variable labels. This is used
// if you want to count the same thing partitioned by various dimensions
// (e.g. HTTP request latencies, partitioned by status code and method). Create
// instances with NewHistogramVec.
type HistogramVec struct {
	*MetricVec
}

// NewHistogramVec creates a new HistogramVec based on the provided HistogramOpts and
// partitioned by the given label names.
func NewHistogramVec(opts HistogramOpts, labelNames []string) *HistogramVec {
	desc := NewDesc(
		BuildFQName(opts.Namespace, opts.Subsystem, opts.Name),
		opts.Help,
		labelNames,
		opts.ConstLabels,
	)
	return &HistogramVec{
		MetricVec: NewMetricVec(desc, func(lvs ...string) Metric {
			return newHistogram(desc, opts, lvs...)
		}),
	}
}

// GetMetricWithLabelValues returns the Histogram for the given slice of label
// values (same order as the variable labels in Desc). If that combination of
// label values is accessed for the first time, a new Histogram is created.
//
// It is possible to call this method without using the returned Histogram to only
// create the new Histogram but leave it at its starting value, a Histogram without
// any observations.
//
// Keeping the Histogram for later use is possible (and should be considered if
// performance is critical), but keep in mind that Reset, DeleteLabelValues and
// Delete can be used to delete the Histogram from the HistogramVec. In that case, the
// Histogram will still exist, but it will not be exported anymore, even if a
// Histogram with the same label values is created later. See also the CounterVec
// example.
//
// An error is returned if the number of label values is not the same as the
// number of variable labels in Desc (minus any curried labels).
//
// Note that for more than one label value, this method is prone to mistakes
// caused by an incorrect order of arguments. Consider GetMetricWith(Labels) as
// an alternative to avoid that type of mistake. For higher label numbers, the
// latter has a much more readable (albeit more verbose) syntax, but it comes
// with a performance overhead (for creating and processing the Labels map).
// See also the GaugeVec example.
func (v *HistogramVec) GetMetricWithLabelValues(lvs ...string) (Observer, error) {
	metric, err := v.MetricVec.GetMetricWithLabelValues(lvs...)
	if metric != nil {
		return metric.(Observer), err
	}
	return nil, err
}

// GetMetricWith returns the Histogram for the given Labels map (the label names
// must match those of the variable labels in Desc). If that label map is
// accessed for the first time, a new Histogram is created. Implications of
// creating a Histogram without using it and keeping the Histogram for later use
// are the same as for GetMetricWithLabelValues.
//
// An error is returned if the number and names of the Labels are inconsistent
// with those of the variable labels in Desc (minus any curried labels).
//
// This method is used for the same purpose as
// GetMetricWithLabelValues(...string). See there for pros and cons of the two
// methods.
func (v *HistogramVec) GetMetricWith(labels Labels) (Observer, error) {
	metric, err := v.MetricVec.GetMetricWith(labels)
	if metric != nil {
		return metric.(Observer), err
	}
	return nil, err
}

// WithLabelValues works as GetMetricWithLabelValues, but panics where
// GetMetricWithLabelValues would have returned an error. Not returning an
// error allows shortcuts like
//     myVec.WithLabelValues("404", "GET").Observe(42.21)
func (v *HistogramVec) WithLabelValues(lvs ...string) Observer {
	h, err := v.GetMetricWithLabelValues(lvs...)
	if err != nil {
		panic(err)
	}
	return h
}

// With works as GetMetricWith but panics where GetMetricWithLabels would have
// returned an error. Not returning an error allows shortcuts like
//     myVec.With(prometheus.Labels{"code": "404", "method": "GET"}).Observe(42.21)
func (v *HistogramVec) With(labels Labels) Observer {
	h, err := v.GetMetricWith(labels)
	if err != nil {
		panic(err)
	}
	return h
}

// CurryWith returns a vector curried with the provided labels, i.e. the
// returned vector has those labels pre-set for all labeled operations performed
// on it. The cardinality of the curried vector is reduced accordingly. The
// order of the remaining labels stays the same (just with the curried labels
// taken out of the sequence – which is relevant for the
// (GetMetric)WithLabelValues methods). It is possible to curry a curried
// vector, but only with labels not yet used for currying before.
//
// The metrics contained in the HistogramVec are shared between the curried and
// uncurried vectors. They are just accessed differently. Curried and uncurried
// vectors behave identically in terms of collection. Only one must be
// registered with a given registry (usually the uncurried version). The Reset
// method deletes all metrics, even if called on a curried vector.
func (v *HistogramVec) CurryWith(labels Labels) (ObserverVec, error) {
	vec, err := v.MetricVec.CurryWith(labels)
	if vec != nil {
		return &HistogramVec{vec}, err
	}
	return nil, err
}

// MustCurryWith works as CurryWith but panics where CurryWith would have
// returned an error.
func (v *HistogramVec) MustCurryWith(labels Labels) ObserverVec {
	vec, err := v.CurryWith(labels)
	if err != nil {
		panic(err)
	}
	return vec
}

type constHistogram struct {
	desc       *Desc
	count      uint64
	sum        float64
	buckets    map[float64]uint64
	labelPairs []*dto.LabelPair
}

func (h *constHistogram) Desc() *Desc {
	return h.desc
}

func (h *constHistogram) Write(out *dto.Metric) error {
	his := &dto.Histogram{}
	buckets := make([]*dto.Bucket, 0, len(h.buckets))

	his.SampleCount = proto.Uint64(h.count)
	his.SampleSum = proto.Float64(h.sum)

	for upperBound, count := range h.buckets {
		buckets = append(buckets, &dto.Bucket{
			CumulativeCount: proto.Uint64(count),
			UpperBound:      proto.Float64(upperBound),
		})
	}

	if len(buckets) > 0 {
		sort.Sort(buckSort(buckets))
	}
	his.Bucket = buckets

	out.Histogram = his
	out.Label = h.labelPairs

	return nil
}

// NewConstHistogram returns a metric representing a Prometheus histogram with
// fixed values for the count, sum, and bucket counts. As those parameters
// cannot be changed, the returned value does not implement the Histogram
// interface (but only the Metric interface). Users of this package will not
// have much use for it in regular operations. However, when implementing custom
// Collectors, it is useful as a throw-away metric that is generated on the fly
// to send it to Prometheus in the Collect method.
//
// buckets is a map of upper bounds to cumulative counts, excluding the +Inf
// bucket.
//
// NewConstHistogram returns an error if the length of labelValues is not
// consistent with the variable labels in Desc or if Desc is invalid.
func NewConstHistogram(
	desc *Desc,
	count uint64,
	sum float64,
	buckets map[float64]uint64,
	labelValues ...string,
) (Metric, error) {
	if desc.err != nil {
		return nil, desc.err
	}
	if err := validateLabelValues(labelValues, len(desc.variableLabels)); err != nil {
		return nil, err
	}
	return &constHistogram{
		desc:       desc,
		count:      count,
		sum:        sum,
		buckets:    buckets,
		labelPairs: MakeLabelPairs(desc, labelValues),
	}, nil
}

// MustNewConstHistogram is a version of NewConstHistogram that panics where
// NewConstHistogram would have returned an error.
func MustNewConstHistogram(
	desc *Desc,
	count uint64,
	sum float64,
	buckets map[float64]uint64,
	labelValues ...string,
) Metric {
	m, err := NewConstHistogram(desc, count, sum, buckets, labelValues...)
	if err != nil {
		panic(err)
	}
	return m
}

type buckSort []*dto.Bucket

func (s buckSort) Len() int {
	return len(s)
}

func (s buckSort) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s buckSort) Less(i, j int) bool {
	return s[i].GetUpperBound() < s[j].GetUpperBound()
}

// pickSparseschema returns the largest number n between -4 and 8 such that
// 2^(2^-n) is less or equal the provided bucketFactor.
//
// Special cases:
//     - bucketFactor <= 1: panics.
//     - bucketFactor < 2^(2^-8) (but > 1): still returns 8.
func pickSparseSchema(bucketFactor float64) int32 {
	if bucketFactor <= 1 {
		panic(fmt.Errorf("bucketFactor %f is <=1", bucketFactor))
	}
	floor := math.Floor(math.Log2(math.Log2(bucketFactor)))
	switch {
	case floor <= -8:
		return 8
	case floor >= 4:
		return -4
	default:
		return -int32(floor)
	}
}
