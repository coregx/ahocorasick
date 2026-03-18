// Package ahocorasick implements the Aho-Corasick algorithm for
// efficient multi-pattern string matching.
//
// The Aho-Corasick algorithm allows searching for multiple patterns
// simultaneously in O(n + m + z) time, where n is the input length,
// m is the total pattern length, and z is the number of matches.
//
// This package is under active development. See README for status.
package ahocorasick

// Version is the current library version.
const Version = "0.2.1"
