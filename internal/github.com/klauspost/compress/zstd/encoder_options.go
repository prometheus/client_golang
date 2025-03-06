package zstd

import (
	"fmt"
	"math"
	"runtime"
	"strings"
)

// EOption is an option for creating a encoder.
type EOption func(*encoderOptions) error

// options retains accumulated state of multiple options.
type encoderOptions struct {
	concurrent      int
	level           EncoderLevel
	single          *bool
	pad             int
	blockSize       int
	windowSize      int
	crc             bool
	fullZero        bool
	noEntropy       bool
	allLitEntropy   bool
	customWindow    bool
	customALEntropy bool
	customBlockSize bool
	lowMem          bool
	dict            *dict
}

func (o *encoderOptions) setDefault() {
	*o = encoderOptions{
		concurrent:    runtime.GOMAXPROCS(0),
		crc:           true,
		single:        nil,
		blockSize:     maxCompressedBlockSize,
		windowSize:    8 << 20,
		level:         SpeedDefault,
		allLitEntropy: false,
		lowMem:        false,
	}
}

// encoder returns an encoder with the selected options.
func (o encoderOptions) encoder() encoder {
	// NOTE(bwplotka): No actual choice to vendor minimal amount of code.
	return &fastEncoderDict{fastEncoder: fastEncoder{fastBase: fastBase{maxMatchOff: int32(o.windowSize), bufferReset: math.MaxInt32 - int32(o.windowSize*2), lowMem: o.lowMem}}}
}

// EncoderLevel predefines encoder compression levels.
// Only use the constants made available, since the actual mapping
// of these values are very likely to change and your compression could change
// unpredictably when upgrading the library.
type EncoderLevel int

const (
	speedNotSet EncoderLevel = iota

	// SpeedFastest will choose the fastest reasonable compression.
	// This is roughly equivalent to the fastest Zstandard mode.
	SpeedFastest

	// SpeedDefault is the default "pretty fast" compression option.
	// This is roughly equivalent to the default Zstandard mode (level 3).
	SpeedDefault

	// SpeedBetterCompression will yield better compression than the default.
	// Currently it is about zstd level 7-8 with ~ 2x-3x the default CPU usage.
	// By using this, notice that CPU usage may go up in the future.
	SpeedBetterCompression

	// SpeedBestCompression will choose the best available compression option.
	// This will offer the best compression no matter the CPU cost.
	SpeedBestCompression

	// speedLast should be kept as the last actual compression option.
	// The is not for external usage, but is used to keep track of the valid options.
	speedLast
)

// EncoderLevelFromString will convert a string representation of an encoding level back
// to a compression level. The compare is not case sensitive.
// If the string wasn't recognized, (false, SpeedDefault) will be returned.
func EncoderLevelFromString(s string) (bool, EncoderLevel) {
	for l := speedNotSet + 1; l < speedLast; l++ {
		if strings.EqualFold(s, l.String()) {
			return true, l
		}
	}
	return false, SpeedDefault
}

// EncoderLevelFromZstd will return an encoder level that closest matches the compression
// ratio of a specific zstd compression level.
// Many input values will provide the same compression level.
func EncoderLevelFromZstd(level int) EncoderLevel {
	switch {
	case level < 3:
		return SpeedFastest
	case level >= 3 && level < 6:
		return SpeedDefault
	case level >= 6 && level < 10:
		return SpeedBetterCompression
	default:
		return SpeedBestCompression
	}
}

// String provides a string representation of the compression level.
func (e EncoderLevel) String() string {
	switch e {
	case SpeedFastest:
		return "fastest"
	case SpeedDefault:
		return "default"
	case SpeedBetterCompression:
		return "better"
	case SpeedBestCompression:
		return "best"
	default:
		return "invalid"
	}
}

// WithEncoderLevel specifies a predefined compression level.
func WithEncoderLevel(l EncoderLevel) EOption {
	return func(o *encoderOptions) error {
		// NOTE(bwlotka): To minimize the vendored size, only SpeedFastest is possible.
		if l != SpeedFastest {
			return fmt.Errorf("unknown encoder level %v", l)
		}
		o.level = l
		if !o.customWindow {
			switch o.level {
			case SpeedFastest:
				o.windowSize = 4 << 20
				if !o.customBlockSize {
					o.blockSize = 1 << 16
				}
			case SpeedDefault:
				o.windowSize = 8 << 20
			case SpeedBetterCompression:
				o.windowSize = 8 << 20
			case SpeedBestCompression:
				o.windowSize = 8 << 20
			}
		}
		if !o.customALEntropy {
			o.allLitEntropy = l > SpeedDefault
		}
		return nil
	}
}
