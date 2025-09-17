// SPDX-FileCopyrightText: Copyright (C) SchedMD LLC.
// SPDX-License-Identifier: Apache-2.0

package domainname

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func Fqdn(name, namespace string) string {
	domain := clusterDomain()
	return fmt.Sprintf("%s.%s.svc.%s", name, namespace, domain)
}

var resolvConfPath = "/etc/resolv.conf"

func clusterDomain() string {
	defaultDomain := "cluster.local"

	file, err := os.Open(resolvConfPath)
	if err != nil {
		return defaultDomain
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "search") {
			continue
		}

		fields := strings.Fields(line)
		for _, domain := range fields[1:] {
			if after, ok := strings.CutPrefix(domain, "svc."); ok {
				return after
			}
		}
	}

	return defaultDomain
}

func FqdnShort(name, namespace string) string {
	return fmt.Sprintf("%s.%s", name, namespace)
}
