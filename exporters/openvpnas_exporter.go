package exporters

import (
	"log"

	"github.com/prometheus/client_golang/prometheus"

	"net"
	"net/http"

	"alexejk.io/go-xmlrpc"
)

type OpenvpnServerHeader struct {
	LabelColumns []string
	Metrics      []OpenvpnServerHeaderField
}

type OpenvpnServerHeaderField struct {
	Column    string
	Desc      *prometheus.Desc
	ValueType prometheus.ValueType
}

type OpenVPNExporter struct {
	xmlrpcPath                                   string
	openvpnUpDesc                                *prometheus.Desc
	openvpnStatusUpdateTimeDesc                  *prometheus.Desc
	openvpnSubscriptionStatusUpdateTimeDesc      *prometheus.Desc
	openvpnSubscriptionCurrentClientConnections  *prometheus.Desc
	openvpnSubscriptionFallbackClientConnections *prometheus.Desc
	openvpnSubscriptionMaximumClientConnections  *prometheus.Desc
	openvpnConnectedClientsDesc                  *prometheus.Desc
}

func NewOpenVPNExporter(xmlrpcPath string) (*OpenVPNExporter, error) {
	// Metrics exported both for client and server statistics.
	openvpnUpDesc := prometheus.NewDesc(
		prometheus.BuildFQName("openvpnas", "", "up"),
		"Whether scraping OpenVPN's metrics was successful.",
		nil, nil)
	openvpnStatusUpdateTimeDesc := prometheus.NewDesc(
		prometheus.BuildFQName("openvpnas", "", "status_update_time_seconds"),
		"UNIX timestamp at which the OpenVPN statistics were updated.",
		nil, nil)

	// Metrics specific to the OpenVPN AS server.
	openvpnConnectedClientsDesc := prometheus.NewDesc(
		prometheus.BuildFQName("openvpnas", "", "server_connected_clients"),
		"Number Of Connected Clients",
		nil, nil)
	openvpnSubscriptionStatusUpdateTimeDesc := prometheus.NewDesc(
		prometheus.BuildFQName("openvpnas", "", "subscription_status_update_time_seconds"),
		"UNIX timestamp at which the OpenVPN subscription status was last updated.",
		nil, nil)
	openvpnSubscriptionCurrentClientConnectionsDesc := prometheus.NewDesc(
		prometheus.BuildFQName("openvpnas", "", "subscription_current_client_connections"),
		"Number of client connections currently being used from the OpenVPN subscription.",
		nil, nil)
	openvpnSubscriptionFallbackClientConnectionsDesc := prometheus.NewDesc(
		prometheus.BuildFQName("openvpnas", "", "subscription_fallback_client_connections"),
		"Number of fallback connections in use on the OpenVPN subscription.",
		nil, nil)
	openvpnSubscriptionMaximumClientConnectionsDesc := prometheus.NewDesc(
		prometheus.BuildFQName("openvpnas", "", "subscription_maximum_client_connections"),
		"Maximum number of client connections allowed by the OpenVPN subscription.",
		nil, nil)

	return &OpenVPNExporter{
		xmlrpcPath:                                   xmlrpcPath,
		openvpnUpDesc:                                openvpnUpDesc,
		openvpnStatusUpdateTimeDesc:                  openvpnStatusUpdateTimeDesc,
		openvpnConnectedClientsDesc:                  openvpnConnectedClientsDesc,
		openvpnSubscriptionStatusUpdateTimeDesc:      openvpnSubscriptionStatusUpdateTimeDesc,
		openvpnSubscriptionCurrentClientConnections:  openvpnSubscriptionCurrentClientConnectionsDesc,
		openvpnSubscriptionFallbackClientConnections: openvpnSubscriptionFallbackClientConnectionsDesc,
		openvpnSubscriptionMaximumClientConnections:  openvpnSubscriptionMaximumClientConnectionsDesc,
	}, nil
}

func (e *OpenVPNExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.openvpnUpDesc
}

func (e *OpenVPNExporter) Collect(ch chan<- prometheus.Metric) {
	dialer := func(_, _ string) (net.Conn, error) {
		return net.Dial("unix", e.xmlrpcPath)
	}

	httpc := http.Client{
		Transport: &http.Transport{
			Dial: dialer,
		},
	}

	opts := []xmlrpc.Option{xmlrpc.HttpClient(&httpc)}
	client, _ := xmlrpc.NewClient("http://localhost/", opts...)
	defer client.Close()


	err := e.CollectVPNSummary(*client, ch)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			e.openvpnUpDesc,
			prometheus.GaugeValue,
			0.0)
		return
	}

	err = e.CollectSubscriptionStatistics(*client, ch)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			e.openvpnUpDesc,
			prometheus.GaugeValue,
			0.0)
		return
	}

	// It should be safe to assume that the service is up if we get this far
	ch <- prometheus.MustNewConstMetric(
		e.openvpnUpDesc,
		prometheus.GaugeValue,
		1.0)
}

func (e *OpenVPNExporter) CollectVPNSummary(client xmlrpc.Client, ch chan<- prometheus.Metric) error {
	result := &struct {
		VPNSummary struct {
			NClients int `xml:"n_clients"`
		}
	}{}

	err := client.Call("GetVPNSummary", nil, result)
	if err != nil {
		log.Printf("Failed to call GetVPNSummary: %s", err)
		return err
	}

	ch <- prometheus.MustNewConstMetric(
		e.openvpnConnectedClientsDesc,
		prometheus.GaugeValue,
		float64(result.VPNSummary.NClients))

	return nil
}

func (e *OpenVPNExporter) CollectSubscriptionStatistics(client xmlrpc.Client, ch chan<- prometheus.Metric) error {
	result := &struct {
		SubscriptionStatus struct {
			AgentDisabled           bool     `xml:"agent_disabled"`
			AgentId                 string   `xml:"agent_id"`
			CcLimit                 int      `xml:"cc_limit"`
			CurrentCc               int      `xml:"current_cc"`
			Error                   string   `xml:"error"`
			FallbackCc              int      `xml:"fallback_cc"`
			GracePeriod             int      `xml:"grace_period"`
			LastSuccessfulUpdate    int      `xml:"last_successful_update"`
			LastSuccessfulUpdateAge int      `xml:"last_successful_update_age"`
			MaxCc                   int      `xml:"max_cc"`
			Name                    string   `xml:"name"`
			NextUpdate              int      `xml:"next_update"`
			NextUpdateIn            int      `xml:"next_update_in"`
			Notes                   []string `xml:"notes"`
			Overdraft               bool     `xml:"overdraft"`
			Server                  string   `xml:"server"`
			State                   string   `xml:"state"`
			Type                    string   `xml:"type"`
			UpdatesFailed           int      `xml:"updates_failed"`
		}
	}{}

	err := client.Call("GetSubscriptionStatus", nil, result)
	if err != nil {
		log.Printf("Failed to call GetSubscriptionStatus: %s", err)
		return err
	}

	ch <- prometheus.MustNewConstMetric(
		e.openvpnSubscriptionStatusUpdateTimeDesc,
		prometheus.GaugeValue,
		float64(result.SubscriptionStatus.LastSuccessfulUpdate))

	ch <- prometheus.MustNewConstMetric(
		e.openvpnSubscriptionCurrentClientConnections,
		prometheus.GaugeValue,
		float64(result.SubscriptionStatus.CurrentCc))

	ch <- prometheus.MustNewConstMetric(
		e.openvpnSubscriptionMaximumClientConnections,
		prometheus.GaugeValue,
		float64(result.SubscriptionStatus.MaxCc))

	ch <- prometheus.MustNewConstMetric(
		e.openvpnSubscriptionFallbackClientConnections,
		prometheus.GaugeValue,
		float64(result.SubscriptionStatus.FallbackCc))

	return nil
}
