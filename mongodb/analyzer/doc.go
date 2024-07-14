// Package analyzer is based on code from the mongoschema project from Facebook (archived 2018, last change 2015)
// BSD-licensed original code: https://github.com/facebookarchive/mongoschema
// See license for this file on [LICENSE]
//
// Main changes:
//
//   - Remove all the CLI interface code, since we're just using it as a library
//   - Remove all the code that generates&outputs Go struct definitions
//   - Add support for more Mongo types, as documented on https://pkg.go.dev/go.mongodb.org/mongo-driver@v1.16.0/bson#hdr-Native_Go_Types
//     (e.g. the original code doesn't support Regex or CodeWithScope fields)
//   - Add the ability to not drill into certain objects, for cases when object keys have been used as identifiers,
//     e.g. {friends: {123: {friended_on: 2024-01-01}, 456: {friended_on: 2024-02-01}}}, where the keys are used as a sort
//     of many-to-many link with intermediate data
package analyzer
