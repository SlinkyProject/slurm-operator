// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"strings"
)

const (
	DefaultSeparator = "="
	DefaultNewline   = true
)

type configBuilder struct {
	sep     string
	newline bool
	props   []configProperty
}

func (b *configBuilder) AddProperty(prop configProperty) *configBuilder {
	b.props = append(b.props, prop)
	return b
}

func (b *configBuilder) WithSeparator(sep string) *configBuilder {
	b.sep = sep
	return b
}

func (b *configBuilder) WithFinalNewline(newline bool) *configBuilder {
	b.newline = newline
	return b
}

func (b *configBuilder) Build() string {
	lines := []string{}
	for _, prop := range b.props {
		if prop.raw {
			lines = append(lines, fmt.Sprintf("%v", prop.val))
		} else {
			lines = append(lines, fmt.Sprintf("%s%s%v", prop.key, b.sep, prop.val))
		}
	}
	if b.newline {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func NewBuilder() *configBuilder {
	return &configBuilder{
		sep:     DefaultSeparator,
		newline: DefaultNewline,
		props:   make([]configProperty, 0),
	}
}

type configProperty struct {
	key string
	val any
	raw bool
}

func NewProperty(key string, val any) configProperty {
	return configProperty{key: key, val: val}
}

func NewPropertyRaw(val any) configProperty {
	return configProperty{val: val, raw: true}
}
