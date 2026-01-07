/*
Copyright 2025 The Kubernetes Authors.

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

package controller

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/route53"
	sd "github.com/aws/aws-sdk-go-v2/service/servicediscovery"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/pkg/crd"
	"sigs.k8s.io/external-dns/pkg/apis/externaldns"
	"sigs.k8s.io/external-dns/pkg/apis/externaldns/validation"
	"sigs.k8s.io/external-dns/pkg/events"
	"sigs.k8s.io/external-dns/pkg/metrics"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
	"sigs.k8s.io/external-dns/provider/akamai"
	"sigs.k8s.io/external-dns/provider/alibabacloud"
	"sigs.k8s.io/external-dns/provider/aws"
	"sigs.k8s.io/external-dns/provider/awssd"
	"sigs.k8s.io/external-dns/provider/azure"
	"sigs.k8s.io/external-dns/provider/civo"
	"sigs.k8s.io/external-dns/provider/cloudflare"
	"sigs.k8s.io/external-dns/provider/coredns"
	"sigs.k8s.io/external-dns/provider/digitalocean"
	"sigs.k8s.io/external-dns/provider/dnsimple"
	"sigs.k8s.io/external-dns/provider/exoscale"
	"sigs.k8s.io/external-dns/provider/gandi"
	"sigs.k8s.io/external-dns/provider/godaddy"
	"sigs.k8s.io/external-dns/provider/google"
	"sigs.k8s.io/external-dns/provider/inmemory"
	"sigs.k8s.io/external-dns/provider/linode"
	"sigs.k8s.io/external-dns/provider/ns1"
	"sigs.k8s.io/external-dns/provider/oci"
	"sigs.k8s.io/external-dns/provider/ovh"
	"sigs.k8s.io/external-dns/provider/pdns"
	"sigs.k8s.io/external-dns/provider/pihole"
	"sigs.k8s.io/external-dns/provider/plural"
	"sigs.k8s.io/external-dns/provider/rfc2136"
	"sigs.k8s.io/external-dns/provider/scaleway"
	"sigs.k8s.io/external-dns/provider/transip"
	"sigs.k8s.io/external-dns/provider/webhook"
	webhookapi "sigs.k8s.io/external-dns/provider/webhook/api"
	"sigs.k8s.io/external-dns/registry"
	"sigs.k8s.io/external-dns/source"
	"sigs.k8s.io/external-dns/source/annotations"
	"sigs.k8s.io/external-dns/source/wrappers"
)

func Execute() {
	cfg := externaldns.NewConfig()
	if err := cfg.ParseFlags(os.Args[1:]); err != nil {
		log.Fatalf("flag parsing error: %v", err)
	}
	log.Infof("config: %s", cfg)
	if err := validation.ValidateConfig(cfg); err != nil {
		log.Fatalf("config validation failed: %v", err)
	}

	// Set annotation prefix (required since init() was removed)
	annotations.SetAnnotationPrefix(cfg.AnnotationPrefix)
	if cfg.AnnotationPrefix != annotations.DefaultAnnotationPrefix {
		log.Infof("Using custom annotation prefix: %s", cfg.AnnotationPrefix)
	}

	configureLogger(cfg)

	if cfg.DryRun {
		log.Info("running in dry-run mode. No changes to DNS records will be made.")
	}

	if log.GetLevel() < log.DebugLevel {
		// Klog V2 is used by k8s.io/apimachinery/pkg/labels and can throw (a lot) of irrelevant logs
		// See https://github.com/kubernetes-sigs/external-dns/issues/2348
		defer klog.ClearLogger()
		klog.SetLogger(logr.Discard())
	}

	log.Info(externaldns.Banner())

	ctx, cancel := context.WithCancel(context.Background())

	go serveMetrics(cfg.MetricsAddress)
	go handleSigterm(cancel)

	endpointsSource, err := buildSource(ctx, cfg)
	if err != nil {
		log.Fatal(err) // nolint: gocritic // exitAfterDefer
	}

	domainFilter := endpoint.NewDomainFilterWithOptions(
		endpoint.WithDomainFilter(cfg.DomainFilter),
		endpoint.WithDomainExclude(cfg.DomainExclude),
		endpoint.WithRegexDomainFilter(cfg.RegexDomainFilter),
		endpoint.WithRegexDomainExclude(cfg.RegexDomainExclude),
	)

	prvdr, err := buildProvider(ctx, cfg, domainFilter)
	if err != nil {
		log.Fatal(err)
	}

	if cfg.WebhookServer {
		webhookapi.StartHTTPApi(prvdr, nil, cfg.WebhookProviderReadTimeout, cfg.WebhookProviderWriteTimeout, "127.0.0.1:8888")
		os.Exit(0)
	}

	ctrl, err := buildController(ctx, cfg, endpointsSource, prvdr, domainFilter)
	if err != nil {
		log.Fatal(err)
	}

	// Register status update callbacks for CRD sources
	if slices.Contains(cfg.Sources, "crd") {
		registerStatusUpdateCallbacks(ctx, ctrl, cfg)
	}

	if cfg.Once {
		err := ctrl.RunOnce(ctx)
		if err != nil {
			log.Fatal(err)
		}

		os.Exit(0)
	}

	if cfg.UpdateEvents {
		// Add RunOnce as the handler function that will be called when ingress/service sources have changed.
		// Note that k8s Informers will perform an initial list operation, which results in the handler
		// function initially being called for every Service/Ingress that exists
		ctrl.Source.AddEventHandler(ctx, func() { ctrl.ScheduleRunOnce(time.Now()) })
	}

	ctrl.ScheduleRunOnce(time.Now())
	ctrl.Run(ctx)
}

func buildProvider(
	ctx context.Context,
	cfg *externaldns.Config,
	domainFilter *endpoint.DomainFilter,
) (provider.Provider, error) {
	var p provider.Provider
	var err error

	zoneNameFilter := endpoint.NewDomainFilter(cfg.ZoneNameFilter)
	zoneIDFilter := provider.NewZoneIDFilter(cfg.ZoneIDFilter)
	zoneTypeFilter := provider.NewZoneTypeFilter(cfg.AWSZoneType)
	zoneTagFilter := provider.NewZoneTagFilter(cfg.AWSZoneTagFilter)

	switch cfg.Provider {
	case "akamai":
		p, err = akamai.NewAkamaiProvider(
			akamai.AkamaiConfig{
				DomainFilter:          domainFilter,
				ZoneIDFilter:          zoneIDFilter,
				ServiceConsumerDomain: cfg.AkamaiServiceConsumerDomain,
				ClientToken:           cfg.AkamaiClientToken,
				ClientSecret:          cfg.AkamaiClientSecret,
				AccessToken:           cfg.AkamaiAccessToken,
				EdgercPath:            cfg.AkamaiEdgercPath,
				EdgercSection:         cfg.AkamaiEdgercSection,
				DryRun:                cfg.DryRun,
			}, nil)
	case "alibabacloud":
		p, err = alibabacloud.NewAlibabaCloudProvider(cfg.AlibabaCloudConfigFile, domainFilter, zoneIDFilter, cfg.AlibabaCloudZoneType, cfg.DryRun)
	case "aws":
		configs := aws.CreateV2Configs(cfg)
		clients := make(map[string]aws.Route53API, len(configs))
		for profile, config := range configs {
			clients[profile] = route53.NewFromConfig(config)
		}

		p, err = aws.NewAWSProvider(
			aws.AWSConfig{
				DomainFilter:          domainFilter,
				ZoneIDFilter:          zoneIDFilter,
				ZoneTypeFilter:        zoneTypeFilter,
				ZoneTagFilter:         zoneTagFilter,
				ZoneMatchParent:       cfg.AWSZoneMatchParent,
				BatchChangeSize:       cfg.AWSBatchChangeSize,
				BatchChangeSizeBytes:  cfg.AWSBatchChangeSizeBytes,
				BatchChangeSizeValues: cfg.AWSBatchChangeSizeValues,
				BatchChangeInterval:   cfg.AWSBatchChangeInterval,
				EvaluateTargetHealth:  cfg.AWSEvaluateTargetHealth,
				PreferCNAME:           cfg.AWSPreferCNAME,
				DryRun:                cfg.DryRun,
				ZoneCacheDuration:     cfg.AWSZoneCacheDuration,
			},
			clients,
		)
	case "aws-sd":
		// Check that only compatible Registry is used with AWS-SD
		if cfg.Registry != "noop" && cfg.Registry != "aws-sd" {
			log.Infof("Registry \"%s\" cannot be used with AWS Cloud Map. Switching to \"aws-sd\".", cfg.Registry)
			cfg.Registry = "aws-sd"
		}
		p, err = awssd.NewAWSSDProvider(domainFilter, cfg.AWSZoneType, cfg.DryRun, cfg.AWSSDServiceCleanup, cfg.TXTOwnerID, cfg.AWSSDCreateTag, sd.NewFromConfig(aws.CreateDefaultV2Config(cfg)))
	case "azure-dns", "azure":
		p, err = azure.NewAzureProvider(cfg.AzureConfigFile, domainFilter, zoneNameFilter, zoneIDFilter, cfg.AzureSubscriptionID, cfg.AzureResourceGroup, cfg.AzureUserAssignedIdentityClientID, cfg.AzureActiveDirectoryAuthorityHost, cfg.AzureZonesCacheDuration, cfg.AzureMaxRetriesCount, cfg.DryRun)
	case "azure-private-dns":
		p, err = azure.NewAzurePrivateDNSProvider(cfg.AzureConfigFile, domainFilter, zoneNameFilter, zoneIDFilter, cfg.AzureSubscriptionID, cfg.AzureResourceGroup, cfg.AzureUserAssignedIdentityClientID, cfg.AzureActiveDirectoryAuthorityHost, cfg.AzureZonesCacheDuration, cfg.AzureMaxRetriesCount, cfg.DryRun)
	case "civo":
		p, err = civo.NewCivoProvider(domainFilter, cfg.DryRun)
	case "cloudflare":
		p, err = cloudflare.NewCloudFlareProvider(
			domainFilter,
			zoneIDFilter,
			cfg.CloudflareProxied,
			cfg.DryRun,
			cloudflare.RegionalServicesConfig{
				Enabled:   cfg.CloudflareRegionalServices,
				RegionKey: cfg.CloudflareRegionKey,
			},
			cloudflare.CustomHostnamesConfig{
				Enabled:              cfg.CloudflareCustomHostnames,
				MinTLSVersion:        cfg.CloudflareCustomHostnamesMinTLSVersion,
				CertificateAuthority: cfg.CloudflareCustomHostnamesCertificateAuthority,
			},
			cloudflare.DNSRecordsConfig{
				PerPage: cfg.CloudflareDNSRecordsPerPage,
				Comment: cfg.CloudflareDNSRecordsComment,
			})
	case "google":
		p, err = google.NewGoogleProvider(ctx, cfg.GoogleProject, domainFilter, zoneIDFilter, cfg.GoogleBatchChangeSize, cfg.GoogleBatchChangeInterval, cfg.GoogleZoneVisibility, cfg.DryRun)
	case "digitalocean":
		p, err = digitalocean.NewDigitalOceanProvider(ctx, domainFilter, cfg.DryRun, cfg.DigitalOceanAPIPageSize)
	case "ovh":
		p, err = ovh.NewOVHProvider(ctx, domainFilter, cfg.OVHEndpoint, cfg.OVHApiRateLimit, cfg.OVHEnableCNAMERelative, cfg.DryRun)
	case "linode":
		p, err = linode.NewLinodeProvider(domainFilter, cfg.DryRun)
	case "dnsimple":
		p, err = dnsimple.NewDnsimpleProvider(domainFilter, zoneIDFilter, cfg.DryRun)
	case "coredns", "skydns":
		p, err = coredns.NewCoreDNSProvider(domainFilter, cfg.CoreDNSPrefix, cfg.TXTOwnerID, cfg.CoreDNSStrictlyOwned, cfg.DryRun)
	case "exoscale":
		p, err = exoscale.NewExoscaleProvider(
			cfg.ExoscaleAPIEnvironment,
			cfg.ExoscaleAPIZone,
			cfg.ExoscaleAPIKey,
			cfg.ExoscaleAPISecret,
			cfg.DryRun,
			exoscale.ExoscaleWithDomain(domainFilter),
			exoscale.ExoscaleWithLogging(),
		)
	case "inmemory":
		p, err = inmemory.NewInMemoryProvider(inmemory.InMemoryInitZones(cfg.InMemoryZones), inmemory.InMemoryWithDomain(domainFilter), inmemory.InMemoryWithLogging()), nil
	case "pdns":
		p, err = pdns.NewPDNSProvider(
			ctx,
			pdns.PDNSConfig{
				DomainFilter: domainFilter,
				DryRun:       cfg.DryRun,
				Server:       cfg.PDNSServer,
				ServerID:     cfg.PDNSServerID,
				APIKey:       cfg.PDNSAPIKey,
				TLSConfig: pdns.TLSConfig{
					SkipTLSVerify:         cfg.PDNSSkipTLSVerify,
					CAFilePath:            cfg.TLSCA,
					ClientCertFilePath:    cfg.TLSClientCert,
					ClientCertKeyFilePath: cfg.TLSClientCertKey,
				},
			},
		)
	case "oci":
		var config *oci.OCIConfig
		// if the instance-principals flag was set, and a compartment OCID was provided, then ignore the
		// OCI config file, and provide a config that uses instance principal authentication.
		if cfg.OCIAuthInstancePrincipal {
			if len(cfg.OCICompartmentOCID) == 0 {
				err = fmt.Errorf("instance principal authentication requested, but no compartment OCID provided")
				break
			}
			authConfig := oci.OCIAuthConfig{UseInstancePrincipal: true}
			config = &oci.OCIConfig{Auth: authConfig, CompartmentID: cfg.OCICompartmentOCID}
		} else {
			if config, err = oci.LoadOCIConfig(cfg.OCIConfigFile); err != nil {
				break
			}
		}
		config.ZoneCacheDuration = cfg.OCIZoneCacheDuration
		p, err = oci.NewOCIProvider(*config, domainFilter, zoneIDFilter, cfg.OCIZoneScope, cfg.DryRun)
	case "rfc2136":
		tlsConfig := rfc2136.TLSConfig{
			UseTLS:                cfg.RFC2136UseTLS,
			SkipTLSVerify:         cfg.RFC2136SkipTLSVerify,
			CAFilePath:            cfg.TLSCA,
			ClientCertFilePath:    cfg.TLSClientCert,
			ClientCertKeyFilePath: cfg.TLSClientCertKey,
		}
		p, err = rfc2136.NewRfc2136Provider(cfg.RFC2136Host, cfg.RFC2136Port, cfg.RFC2136Zone, cfg.RFC2136Insecure, cfg.RFC2136TSIGKeyName, cfg.RFC2136TSIGSecret, cfg.RFC2136TSIGSecretAlg, cfg.RFC2136TAXFR, domainFilter, cfg.DryRun, cfg.RFC2136MinTTL, cfg.RFC2136CreatePTR, cfg.RFC2136GSSTSIG, cfg.RFC2136KerberosUsername, cfg.RFC2136KerberosPassword, cfg.RFC2136KerberosRealm, cfg.RFC2136BatchChangeSize, tlsConfig, cfg.RFC2136LoadBalancingStrategy, nil)
	case "ns1":
		p, err = ns1.NewNS1Provider(
			ns1.NS1Config{
				DomainFilter:  domainFilter,
				ZoneIDFilter:  zoneIDFilter,
				NS1Endpoint:   cfg.NS1Endpoint,
				NS1IgnoreSSL:  cfg.NS1IgnoreSSL,
				DryRun:        cfg.DryRun,
				MinTTLSeconds: cfg.NS1MinTTLSeconds,
			},
		)
	case "transip":
		p, err = transip.NewTransIPProvider(cfg.TransIPAccountName, cfg.TransIPPrivateKeyFile, domainFilter, cfg.DryRun)
	case "scaleway":
		p, err = scaleway.NewScalewayProvider(ctx, domainFilter, cfg.DryRun)
	case "godaddy":
		p, err = godaddy.NewGoDaddyProvider(ctx, domainFilter, cfg.GoDaddyTTL, cfg.GoDaddyAPIKey, cfg.GoDaddySecretKey, cfg.GoDaddyOTE, cfg.DryRun)
	case "gandi":
		p, err = gandi.NewGandiProvider(ctx, domainFilter, cfg.DryRun)
	case "pihole":
		p, err = pihole.NewPiholeProvider(
			pihole.PiholeConfig{
				Server:                cfg.PiholeServer,
				Password:              cfg.PiholePassword,
				TLSInsecureSkipVerify: cfg.PiholeTLSInsecureSkipVerify,
				DomainFilter:          domainFilter,
				DryRun:                cfg.DryRun,
				APIVersion:            cfg.PiholeApiVersion,
			},
		)
	case "plural":
		p, err = plural.NewPluralProvider(cfg.PluralCluster, cfg.PluralProvider)
	case "webhook":
		p, err = webhook.NewWebhookProvider(cfg.WebhookProviderURL)
	default:
		err = fmt.Errorf("unknown dns provider: %s", cfg.Provider)
	}
	if p != nil && cfg.ProviderCacheTime > 0 {
		p = provider.NewCachedProvider(
			p,
			cfg.ProviderCacheTime,
		)
	}
	return p, err
}

func buildController(
	ctx context.Context,
	cfg *externaldns.Config,
	src source.Source,
	p provider.Provider,
	filter *endpoint.DomainFilter,
) (*Controller, error) {
	policy, ok := plan.Policies[cfg.Policy]
	if !ok {
		return nil, fmt.Errorf("unknown policy: %s", cfg.Policy)
	}
	reg, err := registry.SelectRegistry(cfg, p)
	if err != nil {
		return nil, err
	}
	eventsCfg := events.NewConfig(
		events.WithKubeConfig(cfg.KubeConfig, cfg.APIServerURL, cfg.RequestTimeout),
		events.WithEmitEvents(cfg.EmitEvents),
		events.WithDryRun(cfg.DryRun))
	var eventEmitter events.EventEmitter
	if eventsCfg.IsEnabled() {
		eventCtrl, err := events.NewEventController(eventsCfg)
		if err != nil {
			log.Fatal(err)
		}
		eventCtrl.Run(ctx)
		eventEmitter = eventCtrl
	}

	return &Controller{
		Source:                  src,
		Registry:                reg,
		Policy:                  policy,
		Interval:                cfg.Interval,
		DomainFilter:            filter,
		ManagedRecordTypes:      cfg.ManagedDNSRecordTypes,
		ExcludeRecordTypes:      cfg.ExcludeDNSRecordTypes,
		MinEventSyncInterval:    cfg.MinEventSyncInterval,
		TXTOwnerOld:             cfg.TXTOwnerOld,
		EventEmitter:            eventEmitter,
		UpdateDNSEndpointStatus: slices.Contains(cfg.Sources, "crd"),
	}, nil
}

// REFACTORING NOTE: Dual Implementation - 2x2 Matrix
//
// This function supports FOUR combinations for testing/comparison:
//
// DIMENSION 1 - Status Updater Location (STATUS_UPDATER_IMPL):
//   OPTION 1 (pkg-crd):     Uses pkg/crd.StatusUpdater
//   OPTION 2 (controller):  Uses controller.DNSEndpointStatusManager
//
// DIMENSION 2 - Client Type (CLIENT_IMPL):
//   rest (default):          Uses k8s.io/client-go REST client
//   controller-runtime:      Uses sigs.k8s.io/controller-runtime client
//
// TESTING MATRIX (4 combinations):
//   1. STATUS_UPDATER_IMPL=pkg-crd + CLIENT_IMPL=rest (DEFAULT, baseline)
//      → Option 1 with REST client (existing implementation)
//
//   2. STATUS_UPDATER_IMPL=pkg-crd + CLIENT_IMPL=controller-runtime
//      → Option 1 with controller-runtime backed DNSEndpointClient interface
//
//   3. STATUS_UPDATER_IMPL=controller + CLIENT_IMPL=rest
//      → Option 2 with REST client via DNSEndpointClient interface
//
//   4. STATUS_UPDATER_IMPL=controller + CLIENT_IMPL=controller-runtime
//      → Option 2 with direct client.Client usage (no interface)
//
// TO FINALIZE:
//   A. Choose status updater location (Option 1 or 2)
//   B. Choose client type (REST or controller-runtime)
//   C. Remove environment variables
//   D. Delete unused code per file-level comments
//
// registerStatusUpdateCallbacks creates a status updater and registers its callback
func registerStatusUpdateCallbacks(ctx context.Context, ctrl *Controller, cfg *externaldns.Config) {
	// Read environment variable to choose implementation
	// Default to "pkg-crd" (Option 1 - recommended)
	statusUpdaterImpl := os.Getenv("STATUS_UPDATER_IMPL")
	if statusUpdaterImpl == "" {
		statusUpdaterImpl = "pkg-crd"
	}

	log.Infof("Using status updater implementation: %s", statusUpdaterImpl)

	switch statusUpdaterImpl {
	case "pkg-crd":
		// OPTION 1: StatusUpdater in pkg/crd package (RECOMMENDED)
		// Uses pkg/crd.NewDNSEndpointStatusUpdater()
		registerStatusUpdateCallbacksOption1(ctx, ctrl, cfg)

	case "controller":
		// OPTION 2: Status manager in controller package
		// Uses controller.NewDNSEndpointStatusManager()
		registerStatusUpdateCallbacksOption2(ctx, ctrl, cfg)

	default:
		log.Warnf("Unknown STATUS_UPDATER_IMPL value: %s, falling back to pkg-crd", statusUpdaterImpl)
		registerStatusUpdateCallbacksOption1(ctx, ctrl, cfg)
	}
}

// ============================================================================
// OPTION 1: StatusUpdater in pkg/crd package
// ============================================================================
// Supports both REST and controller-runtime client implementations
// TO REMOVE: Delete this function and pkg/crd/status_updater.go
func registerStatusUpdateCallbacksOption1(ctx context.Context, ctrl *Controller, cfg *externaldns.Config) {
	// Check which client implementation to use
	// Default to "rest" for backwards compatibility
	clientImpl := os.Getenv("CLIENT_IMPL")
	if clientImpl == "" {
		clientImpl = "rest"
	}

	var dnsEndpointClient crd.DNSEndpointClient

	if clientImpl == "controller-runtime" {
		// Use controller-runtime backed DNSEndpointClient (NEW)
		log.Info("Option 1: Using controller-runtime backed DNSEndpointClient")

		ctrlRuntimeClient, err := crd.NewControllerRuntimeClient(
			cfg.KubeConfig,
			cfg.APIServerURL,
			cfg.Namespace,
		)
		if err != nil {
			log.Warnf("Could not create controller-runtime client: %v", err)
			return
		}

		dnsEndpointClient = crd.NewDNSEndpointClientCtrlRuntime(ctrlRuntimeClient, cfg.Namespace)
	} else {
		// Use REST client (EXISTING)
		log.Info("Option 1: Using REST client DNSEndpointClient")

		kubeClient, err := getKubeClient(cfg)
		if err != nil {
			log.Warnf("Could not create Kubernetes client: %v", err)
			return
		}

		restClient, _, err := crd.NewCRDClientForAPIVersionKind(
			kubeClient,
			cfg.KubeConfig,
			cfg.APIServerURL,
			cfg.CRDSourceAPIVersion,
			cfg.CRDSourceKind,
		)
		if err != nil {
			log.Warnf("Could not create CRD REST client: %v", err)
			return
		}

		dnsEndpointClient = crd.NewDNSEndpointClient(
			restClient,
			cfg.Namespace,
			cfg.CRDSourceKind,
			metav1.ParameterCodec,
		)
	}

	// Create status updater (Option 1 - service layer in pkg/crd)
	// Uses the DNSEndpointClient interface - works with both implementations
	statusUpdater := crd.NewDNSEndpointStatusUpdater(dnsEndpointClient)

	// Register callback
	callback := func(ctx context.Context, changes *plan.Changes, success bool, message string) {
		dnsEndpoints := extractDNSEndpointsFromChanges(changes)
		log.Debugf("Updating status for %d DNSEndpoint(s)", len(dnsEndpoints))

		for key, ref := range dnsEndpoints {
			err := statusUpdater.UpdateDNSEndpointStatus(ctx, ref.namespace, ref.name, success, message)
			if err != nil {
				log.Warnf("Failed to update status for DNSEndpoint %s: %v", key, err)
			}
		}
	}

	ctrl.RegisterStatusUpdateCallback(callback)
	log.Infof("Registered DNSEndpoint status update callback (Option 1: pkg/crd, client: %s)", clientImpl)
}

// ============================================================================
// OPTION 2: Status manager in controller package
// ============================================================================
// Supports both REST (via interface) and direct controller-runtime implementations
// TO REMOVE: Delete this function and controller/dnsendpoint_status.go
func registerStatusUpdateCallbacksOption2(ctx context.Context, ctrl *Controller, cfg *externaldns.Config) {
	// Check which client implementation to use
	// Default to "rest" for backwards compatibility
	clientImpl := os.Getenv("CLIENT_IMPL")
	if clientImpl == "" {
		clientImpl = "rest"
	}

	if clientImpl == "controller-runtime" {
		// Use direct controller-runtime client (NEW)
		log.Info("Option 2: Using direct controller-runtime client")

		ctrlRuntimeClient, err := crd.NewControllerRuntimeClient(
			cfg.KubeConfig,
			cfg.APIServerURL,
			cfg.Namespace,
		)
		if err != nil {
			log.Warnf("Could not create controller-runtime client: %v", err)
			return
		}

		statusManager := NewDNSEndpointStatusManagerCtrlRuntime(ctrlRuntimeClient)

		callback := func(ctx context.Context, changes *plan.Changes, success bool, message string) {
			dnsEndpoints := extractDNSEndpointsFromChanges(changes)
			log.Debugf("Updating status for %d DNSEndpoint(s) (controller-runtime direct)", len(dnsEndpoints))

			for key, ref := range dnsEndpoints {
				err := statusManager.UpdateStatus(ctx, ref.namespace, ref.name, success, message)
				if err != nil {
					log.Warnf("Failed to update status for DNSEndpoint %s: %v", key, err)
				}
			}
		}

		ctrl.RegisterStatusUpdateCallback(callback)
		log.Infof("Registered DNSEndpoint status update callback (Option 2: controller, client: controller-runtime)")
	} else {
		// Use REST client via DNSEndpointClient interface (EXISTING)
		log.Info("Option 2: Using REST client via DNSEndpointClient interface")

		kubeClient, err := getKubeClient(cfg)
		if err != nil {
			log.Warnf("Could not create Kubernetes client: %v", err)
			return
		}

		restClient, _, err := crd.NewCRDClientForAPIVersionKind(
			kubeClient,
			cfg.KubeConfig,
			cfg.APIServerURL,
			cfg.CRDSourceAPIVersion,
			cfg.CRDSourceKind,
		)
		if err != nil {
			log.Warnf("Could not create CRD REST client: %v", err)
			return
		}

		dnsEndpointClient := crd.NewDNSEndpointClient(
			restClient,
			cfg.Namespace,
			cfg.CRDSourceKind,
			metav1.ParameterCodec,
		)

		statusManager := NewDNSEndpointStatusManager(dnsEndpointClient)

		callback := func(ctx context.Context, changes *plan.Changes, success bool, message string) {
			dnsEndpoints := extractDNSEndpointsFromChanges(changes)
			log.Debugf("Updating status for %d DNSEndpoint(s)", len(dnsEndpoints))

			for key, ref := range dnsEndpoints {
				err := statusManager.UpdateStatus(ctx, ref.namespace, ref.name, success, message)
				if err != nil {
					log.Warnf("Failed to update status for DNSEndpoint %s: %v", key, err)
				}
			}
		}

		ctrl.RegisterStatusUpdateCallback(callback)
		log.Infof("Registered DNSEndpoint status update callback (Option 2: controller, client: %s)", clientImpl)
	}
}

// getKubeClient creates a Kubernetes client from config
// Helper function used by both Option 1 and Option 2
func getKubeClient(cfg *externaldns.Config) (kubernetes.Interface, error) {
	clientGen := &source.SingletonClientGenerator{
		KubeConfig:     cfg.KubeConfig,
		APIServerURL:   cfg.APIServerURL,
		RequestTimeout: cfg.RequestTimeout,
	}
	return clientGen.KubeClient()
}

// dnsEndpointRef holds a reference to a DNSEndpoint CRD
type dnsEndpointRef struct {
	namespace string
	name      string
}

// extractDNSEndpointsFromChanges extracts unique DNSEndpoint references from plan changes
func extractDNSEndpointsFromChanges(changes *plan.Changes) map[string]dnsEndpointRef {
	endpoints := make(map[string]dnsEndpointRef)

	// Helper to add endpoint ref
	addEndpoint := func(ep *endpoint.Endpoint) {
		if ep == nil {
			return
		}
		ref := ep.RefObject()
		if ref == nil || ref.Kind != "DNSEndpoint" {
			return
		}
		key := fmt.Sprintf("%s/%s", ref.Namespace, ref.Name)
		if _, exists := endpoints[key]; !exists {
			endpoints[key] = dnsEndpointRef{
				namespace: ref.Namespace,
				name:      ref.Name,
			}
		}
	}

	// Collect from all change types
	for _, ep := range changes.Create {
		addEndpoint(ep)
	}
	for _, ep := range changes.UpdateOld {
		addEndpoint(ep)
	}
	for _, ep := range changes.UpdateNew {
		addEndpoint(ep)
	}
	for _, ep := range changes.Delete {
		addEndpoint(ep)
	}

	return endpoints
}

// This function configures the logger format and level based on the provided configuration.
func configureLogger(cfg *externaldns.Config) {
	if cfg.LogFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}
	ll, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatalf("failed to parse log level: %v", err)
	}
	log.SetLevel(ll)
}

// buildSource creates and configures the source(s) for endpoint discovery based on the provided configuration.
// It initializes the source configuration, generates the required sources, and combines them into a single,
// deduplicated source. Returns the combined source or an error if source creation fails.
func buildSource(ctx context.Context, cfg *externaldns.Config) (source.Source, error) {
	sourceCfg := source.NewSourceConfig(cfg)
	sources, err := source.ByNames(ctx, &source.SingletonClientGenerator{
		KubeConfig:   cfg.KubeConfig,
		APIServerURL: cfg.APIServerURL,
		RequestTimeout: func() time.Duration {
			if cfg.UpdateEvents {
				return 0
			}
			return cfg.RequestTimeout
		}(),
	}, cfg.Sources, sourceCfg)
	if err != nil {
		return nil, err
	}
	opts := wrappers.NewConfig(
		wrappers.WithDefaultTargets(cfg.DefaultTargets),
		wrappers.WithForceDefaultTargets(cfg.ForceDefaultTargets),
		wrappers.WithNAT64Networks(cfg.NAT64Networks),
		wrappers.WithTargetNetFilter(cfg.TargetNetFilter),
		wrappers.WithExcludeTargetNets(cfg.ExcludeTargetNets),
		wrappers.WithMinTTL(cfg.MinTTL))
	return wrappers.WrapSources(sources, opts)
}

// handleSigterm listens for a SIGTERM signal and triggers the provided cancel function
// to gracefully terminate the application. It logs a message when the signal is received.
func handleSigterm(cancel func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	<-signals
	log.Info("Received SIGTERM. Terminating...")
	cancel()
}

// serveMetrics starts an HTTP server that serves health and metrics endpoints.
// The /healthz endpoint returns a 200 OK status to indicate the service is healthy.
// The /metrics endpoint serves Prometheus metrics.
// The server listens on the specified address and logs debug information about the endpoints.
func serveMetrics(address string) {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	log.Debugf("serving 'healthz' on '%s/healthz'", address)
	log.Debugf("serving 'metrics' on '%s/metrics'", address)
	log.Debugf("registered '%d' metrics", len(metrics.RegisterMetric.Metrics))

	http.Handle("/metrics", promhttp.Handler())

	log.Fatal(http.ListenAndServe(address, nil))
}
