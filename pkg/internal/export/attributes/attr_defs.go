package attributes

import (
	"maps"

	attr "github.com/grafana/beyla/pkg/internal/export/attributes/names"
)

// AttrGroups will let enabling by default some groups of attributes under
// given circumstances. For example, will let enabling kubernetes metadata attributes
// only if Beyla is running under Kubernetes and kube metadata is enabled.
type AttrGroups int

const (
	GroupKubernetes = AttrGroups(1 << iota)
	GroupPrometheus
	GroupHTTPRoutes
	GroupNetIfaceDirection
	GroupNetCIDR
	GroupPeerInfo // TODO Beyla 2.0: remove when we remove ReportPeerInfo configuration option
	GroupTarget   // TODO Beyla 2.0: remove when we remove ReportTarget configuration option
	GroupTraces
)

func (e *AttrGroups) Has(groups AttrGroups) bool {
	return *e&groups != 0
}

func (e *AttrGroups) Add(groups AttrGroups) {
	*e |= groups
}

// Any new metric and attribute must be added here to be matched from the user-provided wildcard
// selectors of the attributes.select section
func getDefinitions(groups AttrGroups) map[Section]AttrReportGroup {
	kubeEnabled := groups.Has(GroupKubernetes)
	promEnabled := groups.Has(GroupPrometheus)
	ifaceDirEnabled := groups.Has(GroupNetIfaceDirection)
	peerInfoEnabled := groups.Has(GroupPeerInfo)
	cidrEnabled := groups.Has(GroupNetCIDR)

	// attributes to be reported exclusively for prometheus exporters
	var prometheusAttributes = AttrReportGroup{
		Disabled: !promEnabled,
		Attributes: map[attr.Name]Default{
			attr.TargetInstance:   true,
			attr.ServiceNamespace: true,
		},
	}
	// ServiceName is reported both as resource and metrics attribute, as
	// the OTEL definition requires that it is reported as resource attribute
	// but Grafana Cloud takes int from the metric
	var appAttributes = AttrReportGroup{
		SubGroups: []*AttrReportGroup{&prometheusAttributes},
		Attributes: map[attr.Name]Default{
			attr.ServiceName: true,
		},
	}

	// attributes to be reported exclusively for network metrics when
	// kubernetes metadata is enabled
	var networkKubeAttributes = AttrReportGroup{
		Disabled: !kubeEnabled,
		Attributes: map[attr.Name]Default{
			attr.K8sSrcOwnerName: true,
			attr.K8sSrcNamespace: true,
			attr.K8sDstOwnerName: true,
			attr.K8sDstNamespace: true,
			attr.K8sClusterName:  true,
			attr.K8sSrcName:      false,
			attr.K8sSrcType:      false,
			attr.K8sSrcOwnerType: false,
			attr.K8sSrcNodeIP:    false,
			attr.K8sSrcNodeName:  false,
			attr.K8sDstName:      false,
			attr.K8sDstType:      false,
			attr.K8sDstOwnerType: false,
			attr.K8sDstNodeIP:    false,
			attr.K8sDstNodeName:  false,
		},
	}

	// network CIDR attributes are only enabled if the CIDRs configuration
	// is defined
	var networkCIDR = AttrReportGroup{
		Disabled: !cidrEnabled,
		Attributes: map[attr.Name]Default{
			attr.DstCIDR: true,
			attr.SrcCIDR: true,
		},
	}

	// attributes to be reported exclusively for application metrics when
	// kubernetes metadata is enabled
	var appKubeAttributes = AttrReportGroup{
		Disabled: !kubeEnabled,
		Attributes: map[attr.Name]Default{
			attr.K8sNamespaceName:   true,
			attr.K8sPodName:         true,
			attr.K8sDeploymentName:  true,
			attr.K8sReplicaSetName:  true,
			attr.K8sDaemonSetName:   true,
			attr.K8sStatefulSetName: true,
			attr.K8sNodeName:        true,
			attr.K8sPodUID:          true,
			attr.K8sPodStartTime:    true,
		},
	}

	var httpRoutes = AttrReportGroup{
		Disabled: !groups.Has(GroupHTTPRoutes),
		Attributes: map[attr.Name]Default{
			attr.HTTPRoute: true,
		},
	}

	var serverInfo = AttrReportGroup{
		Attributes: map[attr.Name]Default{
			attr.ClientAddr: Default(peerInfoEnabled),
		},
	}
	var httpClientInfo = AttrReportGroup{
		Attributes: map[attr.Name]Default{
			attr.ServerAddr: Default(peerInfoEnabled),
			attr.ServerPort: Default(peerInfoEnabled),
		},
	}
	var grpcClientInfo = AttrReportGroup{
		Attributes: map[attr.Name]Default{
			attr.ServerAddr: Default(peerInfoEnabled),
		},
	}

	// TODO Beyla 2.0 remove
	// this just defaults the path as default when the target report is enabled
	// via the deprecated BEYLA_METRICS_REPORT_PEER config option
	var deprecatedHTTPPath = AttrReportGroup{
		Disabled: !groups.Has(GroupTarget),
		Attributes: map[attr.Name]Default{
			attr.HTTPUrlPath: true,
		},
	}

	var httpCommon = AttrReportGroup{
		SubGroups: []*AttrReportGroup{&httpRoutes, &deprecatedHTTPPath},
		Attributes: map[attr.Name]Default{
			attr.HTTPRequestMethod:      true,
			attr.HTTPResponseStatusCode: true,
			attr.HTTPUrlPath:            false,
		},
	}

	return map[Section]AttrReportGroup{
		BeylaNetworkFlow.Section: {
			SubGroups: []*AttrReportGroup{&networkCIDR, &networkKubeAttributes},
			Attributes: map[attr.Name]Default{
				attr.BeylaIP:    false,
				attr.Transport:  false,
				attr.SrcAddress: false,
				attr.DstAddres:  false,
				attr.SrcPort:    false,
				attr.DstPort:    false,
				attr.SrcName:    false,
				attr.DstName:    false,
				attr.Direction:  Default(ifaceDirEnabled),
				attr.Iface:      Default(ifaceDirEnabled),
			},
		},
		HTTPServerDuration.Section: {
			SubGroups: []*AttrReportGroup{&appAttributes, &appKubeAttributes, &httpCommon, &serverInfo},
		},
		HTTPServerRequestSize.Section: {
			SubGroups: []*AttrReportGroup{&appAttributes, &appKubeAttributes, &httpCommon, &serverInfo},
		},
		HTTPClientDuration.Section: {
			SubGroups: []*AttrReportGroup{&appAttributes, &appKubeAttributes, &httpCommon, &httpClientInfo},
		},
		HTTPClientRequestSize.Section: {
			SubGroups: []*AttrReportGroup{&appAttributes, &appKubeAttributes, &httpCommon, &httpClientInfo},
		},
		RPCClientDuration.Section: {
			SubGroups: []*AttrReportGroup{&appAttributes, &appKubeAttributes, &grpcClientInfo},
			Attributes: map[attr.Name]Default{
				attr.RPCMethod:         true,
				attr.RPCSystem:         true,
				attr.RPCGRPCStatusCode: true,
			},
		},
		RPCServerDuration.Section: {
			SubGroups: []*AttrReportGroup{&appAttributes, &appKubeAttributes, &serverInfo},
			Attributes: map[attr.Name]Default{
				attr.RPCMethod:         true,
				attr.RPCSystem:         true,
				attr.RPCGRPCStatusCode: true,
				// Overriding default serverInfo configuration because we want
				// to report it by default
				attr.ClientAddr: true,
			},
		},
		DBClientDuration.Section: {
			SubGroups: []*AttrReportGroup{&appAttributes, &appKubeAttributes},
			Attributes: map[attr.Name]Default{
				attr.DBOperation: true,
				attr.DBSystem:    true,
			},
		},
		Traces.Section: {
			Attributes: map[attr.Name]Default{
				attr.DBQueryText: false,
			},
		},
	}
}

// AllAttributeNames returns a set with all the names in the attributes database
// as returned by the getDefinitions function
func AllAttributeNames() map[attr.Name]struct{} {
	names := map[attr.Name]struct{}{}
	// -1 to enable all the metric group flags
	for _, section := range getDefinitions(-1) {
		maps.Copy(names, section.All())
	}
	return names
}
