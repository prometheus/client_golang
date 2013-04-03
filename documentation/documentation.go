// Copyright (c) 2013, Prometheus Team
// All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can be found
// in the LICENSE file.

// A repository of various Prometheus client documentation and advice.
//
// Please try to observe the following rules when naming metrics:
//
// - Use underbars "_" to separate words.
//
// - Have the metric name start from generality and work toward specificity
// toward the end.  For example, when working with multiple caching subsystems,
// consider using the following structure "cache" + "user_credentials" →
// "cache_user_credentials" and "cache" + "value_transformations" →
// "cache_value_transformations".
//
// - Have whatever is being measured follow the system and subsystem names cited
// supra.  For instance, with "insertions", "deletions", "evictions",
// "replacements" of the above cache, they should be named as
// "cache_user_credentials_insertions" and "cache_user_credentials_deletions"
// and "cache_user_credentials_deletions" and
// "cache_user_credentials_evictions".
//
// - If what is being measured has a standardized unit around it, consider
// providing a unit for it.
//
// - Consider adding an additional suffix that designates what the value
// represents such as a "total" or "size"---e.g.,
// "cache_user_credentials_size_kb" or
// "cache_user_credentials_insertions_total".
//
// - Give heed to how future-proof the names are.  Things may depend on these
// names; and as your service evolves, the calculated values may take on
// different meanings, which can be difficult to reflect if deployed code
// depends on antique names.
//
// Further considerations:
//
// - The Registry's exposition mechanism is not backed by authorization and
// authentication.  This is something that will need to be addressed for
// production services that are directly exposed to the outside world.
//
// - Engage in as little in-process processing of values as possible.  The job
// of processing and aggregation of these values belongs in a separate
// post-processing job.  The same goes for archiving.  I will need to evaluate
// hooks into something like OpenTSBD.
package documentation
