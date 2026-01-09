/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provider

import (
	"strconv"

	log "github.com/sirupsen/logrus"
)

// This file contains safe type conversion utilities that handle errors gracefully
// by logging and returning default values. This reduces boilerplate across providers.

// ParseInt64 safely parses a string to int64, returning defaultVal on error.
// Logs a warning with the provided context if parsing fails.
func ParseInt64(value string, defaultVal int64, context string) int64 {
	val, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		log.Warnf("Failed parsing %s: %q: %v; using default %d", context, value, err, defaultVal)
		return defaultVal
	}
	return val
}

// ParseInt32 safely parses a string to int32, returning defaultVal on error.
// Logs a warning with the provided context if parsing fails.
func ParseInt32(value string, defaultVal int32, context string) int32 {
	val, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		log.Warnf("Failed parsing %s: %q: %v; using default %d", context, value, err, defaultVal)
		return defaultVal
	}
	return int32(val)
}

// ParseUint32 safely parses a string to uint32, returning defaultVal on error.
// Logs a warning with the provided context if parsing fails.
func ParseUint32(value string, defaultVal uint32, context string) uint32 {
	val, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		log.Warnf("Failed parsing %s: %q: %v; using default %d", context, value, err, defaultVal)
		return defaultVal
	}
	return uint32(val)
}

// ParseFloat64 safely parses a string to float64, returning defaultVal on error.
// Logs a warning with the provided context if parsing fails.
func ParseFloat64(value string, defaultVal float64, context string) float64 {
	val, err := strconv.ParseFloat(value, 64)
	if err != nil {
		log.Warnf("Failed parsing %s: %q: %v; using default %f", context, value, err, defaultVal)
		return defaultVal
	}
	return val
}

// ParseBool safely parses a string to bool, returning defaultVal on error.
// Logs a warning with the provided context if parsing fails.
func ParseBool(value string, defaultVal bool, context string) bool {
	val, err := strconv.ParseBool(value)
	if err != nil {
		log.Warnf("Failed parsing %s: %q: %v; using default %t", context, value, err, defaultVal)
		return defaultVal
	}
	return val
}

// ParseInt64OrError parses a string to int64, returning an error if parsing fails.
// Use this when you need to handle the error explicitly rather than using a default.
func ParseInt64OrError(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

// ParseFloat64OrError parses a string to float64, returning an error if parsing fails.
// Use this when you need to handle the error explicitly rather than using a default.
func ParseFloat64OrError(value string) (float64, error) {
	return strconv.ParseFloat(value, 64)
}
