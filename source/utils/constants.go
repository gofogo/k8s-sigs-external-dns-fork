package utils

import "math"

const (
	// CloudflareProxiedKey TODO: move contants to a specific folder like source/cloudflare/constants.go

	// CloudflareProxiedKey The annotation used for determining if traffic will go through Cloudflare
	CloudflareProxiedKey        = "external-dns.alpha.kubernetes.io/cloudflare-proxied"
	CloudflareCustomHostnameKey = "external-dns.alpha.kubernetes.io/cloudflare-custom-hostname"

	SetIdentifierKey   = "external-dns.alpha.kubernetes.io/set-identifier"
	aliasAnnotationKey = "external-dns.alpha.kubernetes.io/alias"

	targetAnnotationKey = "external-dns.alpha.kubernetes.io/target"

	ttlAnnotationKey = "external-dns.alpha.kubernetes.io/ttl"
	ttlMinimum       = 1
	ttlMaximum       = math.MaxInt32
)
