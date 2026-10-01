package web

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	transactionTaxFinancePath = "/WebObjects/iTunesConnect.woa/ra/paymentConsolidation"
	transactionTaxPollLimit   = 120
	transactionTaxPollWait    = 5 * time.Second
)

func isTransactionTaxFinanceRequestURL(value *url.URL) bool {
	if value == nil {
		return false
	}
	return value.Path == transactionTaxFinancePath || strings.HasPrefix(value.Path, transactionTaxFinancePath+"/")
}

// TransactionTaxReportRequest identifies the provider and accounting month
// for a Transaction Tax Report generation request. ProviderID is the numeric
// provider selected by the authenticated web session.
type TransactionTaxReportRequest struct {
	ProviderID int64
	Date       string
}

// TransactionTaxReportDownload contains the ready report stream and safe
// response metadata. The generated job ID and signed download URL are kept
// inside the web client and are never returned to command output.
type TransactionTaxReportDownload struct {
	Body                      io.ReadCloser
	PollStatus                string
	ContentType               string
	ContentDispositionPresent bool
}

type transactionTaxVendor struct {
	SAPVendorNumber int64
	LastSoldUnit    float64
	IsArcadeVendor  bool
}

type transactionTaxRegionCurrency struct {
	ID            int64
	RegionCode    string
	RegionNameKey string
	RegionName    string
}

type transactionTaxProceed struct {
	RegionCurrency      *transactionTaxRegionCurrency
	FinancialReportType string
	Earned              json.RawMessage
}

type transactionTaxReportSummary struct {
	Proceeds []transactionTaxProceed `json:"proceedsByRegion"`
}

type transactionTaxMonth struct {
	HasVendorTaxReport bool
	IsArcadeVendor     bool
	Summaries          []transactionTaxReportSummary
}

type transactionTaxRegionGroup struct {
	SortName string
	IDs      []int64
}

// DownloadTransactionTaxReport mirrors the authenticated finance page's
// private generation flow: resolve the default SAP vendor, read the selected
// month, derive the UI's all-region list, generate once, poll the job, and
// open the same-origin ready artifact once.
func (c *Client) DownloadTransactionTaxReport(ctx context.Context, request TransactionTaxReportRequest) (*TransactionTaxReportDownload, error) {
	if c == nil || c.httpClient == nil {
		return nil, errors.New("transaction tax web client is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	year, month, err := normalizeTransactionTaxDate(request.Date)
	if err != nil {
		return nil, err
	}
	if request.ProviderID <= 0 {
		return nil, errors.New("transaction tax provider is required")
	}
	origin, err := c.transactionTaxOrigin()
	if err != nil {
		return nil, err
	}

	localizationBody, err := c.transactionTaxJSON(ctx, transactionTaxFinancePath+"/localization/keyValueMapping")
	if err != nil {
		return nil, transactionTaxOperationError("localization request", err)
	}
	localization, err := decodeTransactionTaxLocalization(localizationBody)
	if err != nil {
		return nil, transactionTaxProtocolError("localization response")
	}

	// The finance page loads this mapping before constructing its region model.
	// Its values are not needed in the generation query, so the response is
	// deliberately discarded after the successful read.
	if _, err := c.transactionTaxJSON(ctx, transactionTaxFinancePath+"/regionCountriesMapping"); err != nil {
		return nil, transactionTaxOperationError("region mapping request", err)
	}

	vendorBody, err := c.transactionTaxJSON(ctx, fmt.Sprintf(
		"%s/providers/%d/sapVendorNumbers", transactionTaxFinancePath, request.ProviderID,
	))
	if err != nil {
		return nil, transactionTaxOperationError("vendor request", err)
	}
	vendors, err := decodeTransactionTaxVendors(vendorBody)
	if err != nil {
		return nil, transactionTaxProtocolError("vendor response")
	}
	vendor, err := selectTransactionTaxVendor(vendors)
	if err != nil {
		return nil, err
	}

	monthBody, err := c.transactionTaxJSON(ctx, fmt.Sprintf(
		"%s/providers/%d/sapVendorNumbers/%d?year=%s&month=%s",
		transactionTaxFinancePath, request.ProviderID, vendor.SAPVendorNumber, year, month,
	))
	if err != nil {
		return nil, transactionTaxOperationError("period request", err)
	}
	selectedMonth, err := decodeTransactionTaxMonth(monthBody)
	if err != nil {
		return nil, transactionTaxProtocolError("period response")
	}
	if !selectedMonth.HasVendorTaxReport {
		return nil, errors.New("transaction tax report is not available for the requested period")
	}
	if selectedMonth.IsArcadeVendor || vendor.IsArcadeVendor {
		return nil, errors.New("transaction tax report has no region currencies for the requested period")
	}
	regionIDs, err := deriveTransactionTaxRegionIDs(selectedMonth, localization)
	if err != nil {
		return nil, err
	}

	jobID, err := c.generateTransactionTaxReport(ctx, request.ProviderID, vendor.SAPVendorNumber, year, month, regionIDs)
	if err != nil {
		return nil, err
	}
	status, downloadURL, err := c.pollTransactionTaxReport(ctx, request.ProviderID, vendor.SAPVendorNumber, jobID)
	if err != nil {
		return nil, err
	}

	download, err := c.openTransactionTaxDownload(ctx, origin, downloadURL)
	if err != nil {
		return nil, err
	}
	download.PollStatus = status
	return download, nil
}

func normalizeTransactionTaxDate(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if len(value) != len("2006-01") || value[4] != '-' {
		return "", "", errors.New("--date must use YYYY-MM")
	}
	parsed, err := time.Parse("2006-01", value)
	if err != nil || parsed.Format("2006-01") != value {
		return "", "", errors.New("--date must use a valid YYYY-MM month")
	}
	return value[:4], strconv.Itoa(int(parsed.Month())), nil
}

func (c *Client) transactionTaxOrigin() (*url.URL, error) {
	base := appStoreBaseURL
	if c != nil && strings.TrimSpace(c.baseURL) != "" {
		base = strings.TrimSpace(c.baseURL)
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("transaction tax web origin is invalid")
	}
	if !strings.EqualFold(parsed.Scheme, "https") {
		return nil, errors.New("transaction tax web origin must use HTTPS")
	}
	return &url.URL{Scheme: parsed.Scheme, Host: parsed.Host}, nil
}

func (c *Client) transactionTaxJSON(ctx context.Context, path string) ([]byte, error) {
	return c.transactionTaxJSONWithHTTPClient(ctx, path, c.httpClient)
}

func (c *Client) transactionTaxJSONWithHTTPClient(ctx context.Context, path string, client *http.Client) ([]byte, error) {
	origin, err := c.transactionTaxOrigin()
	if err != nil {
		return nil, err
	}
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	headers.Set("Origin", origin.String())
	headers.Set("Referer", origin.String()+"/itc/payments_and_financial_reports/")
	headers.Set("X-Requested-With", "XMLHttpRequest")
	return c.doRequestBaseWithHTTPClient(client, ctx, origin.String(), http.MethodGet, path, nil, headers)
}

func transactionTaxNoRetryHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		return nil
	}
	clone := *client
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	base, ok := transport.(*http.Transport)
	if !ok {
		// A custom RoundTripper owns its own retry policy. Preserve it while
		// avoiding any changes to the caller's client.
		return &clone
	}

	noRetryTransport := base.Clone()
	noRetryTransport.DisableKeepAlives = true
	noRetryTransport.ForceAttemptHTTP2 = false
	noRetryTransport.TLSNextProto = make(map[string]func(string, *tls.Conn) http.RoundTripper)
	http1Only := new(http.Protocols)
	http1Only.SetHTTP1(true)
	noRetryTransport.Protocols = http1Only
	if tlsConfig := noRetryTransport.TLSClientConfig; tlsConfig != nil {
		filteredNextProtos := make([]string, 0, len(tlsConfig.NextProtos)+1)
		hasHTTP1 := false
		for _, protocol := range tlsConfig.NextProtos {
			if protocol == "h2" {
				continue
			}
			if protocol == "http/1.1" {
				hasHTTP1 = true
			}
			filteredNextProtos = append(filteredNextProtos, protocol)
		}
		if !hasHTTP1 {
			filteredNextProtos = append(filteredNextProtos, "http/1.1")
		}
		tlsConfig.NextProtos = filteredNextProtos
	}
	clone.Transport = noRetryTransport
	return &clone
}

func transactionTaxSingleAttemptHTTPClient(client *http.Client) *http.Client {
	clone := transactionTaxNoRetryHTTPClient(client)
	if clone == nil {
		return nil
	}
	clone.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return clone
}

func (c *Client) generateTransactionTaxReport(ctx context.Context, providerID, vendorID int64, year, month string, regionIDs []int64) (string, error) {
	if len(regionIDs) == 0 {
		return "", errors.New("transaction tax report has no region currencies for the requested period")
	}
	joined := make([]string, len(regionIDs))
	for i, id := range regionIDs {
		joined[i] = strconv.FormatInt(id, 10)
	}
	path := fmt.Sprintf(
		"%s/providers/%d/sapVendorNumbers/%d/reports?year=%s&month=%s&regionCurrencyIds=%s&reportTypes=&isVendorTaxReportReq=true",
		transactionTaxFinancePath, providerID, vendorID, year, month, strings.Join(joined, ","),
	)
	body, err := c.transactionTaxJSONWithHTTPClient(ctx, path, transactionTaxSingleAttemptHTTPClient(c.httpClient))
	if err != nil {
		return "", transactionTaxOperationError("report generation", err)
	}
	object, err := decodeTransactionTaxObject(body)
	if err != nil {
		return "", transactionTaxProtocolError("report generation response")
	}
	uuid, err := transactionTaxStringField(object, "uuid")
	uuid = strings.TrimSpace(uuid)
	if err != nil || uuid == "" {
		return "", transactionTaxProtocolError("report generation response")
	}
	if _, err := transactionTaxStrictIntegerField(object, "estimatedWaitingTime"); err != nil {
		return "", transactionTaxProtocolError("report generation response")
	}
	return uuid, nil
}

var transactionTaxPollWaitFn = func(ctx context.Context) error {
	timer := time.NewTimer(transactionTaxPollWait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) pollTransactionTaxReport(ctx context.Context, providerID, vendorID int64, jobID string) (string, string, error) {
	for attempt := 0; attempt < transactionTaxPollLimit; attempt++ {
		if attempt > 0 {
			if err := transactionTaxPollWaitFn(ctx); err != nil {
				return "", "", transactionTaxUnknownError("status polling", err)
			}
		}
		body, err := c.transactionTaxJSON(ctx, fmt.Sprintf(
			"%s/providers/%d/sapVendorNumbers/%d/reports/%s/status",
			transactionTaxFinancePath, providerID, vendorID, url.PathEscape(jobID),
		))
		if err != nil {
			if ctx != nil && ctx.Err() != nil {
				return "", "", transactionTaxUnknownError("status polling", ctx.Err())
			}
			return "", "", transactionTaxUnknownError("status polling", err)
		}
		object, err := decodeTransactionTaxObject(body)
		if err != nil {
			return "", "", transactionTaxProtocolError("status response")
		}
		status, err := transactionTaxStringField(object, "status")
		if err != nil || strings.TrimSpace(status) == "" {
			return "", "", transactionTaxProtocolError("status response")
		}
		if status != "readyForDownload" {
			continue
		}
		downloadURL, err := transactionTaxStringField(object, "downloadUrl")
		if err != nil || strings.TrimSpace(downloadURL) == "" {
			return "", "", transactionTaxProtocolError("ready status response")
		}
		return status, downloadURL, nil
	}
	return "", "", transactionTaxUnknownError("status polling", errors.New("poll limit reached before readiness"))
}

func (c *Client) openTransactionTaxDownload(ctx context.Context, origin *url.URL, rawDownloadURL string) (*TransactionTaxReportDownload, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	target, err := url.Parse(strings.TrimSpace(rawDownloadURL))
	if err != nil || strings.TrimSpace(rawDownloadURL) == "" {
		return nil, errors.New("transaction tax download URL is invalid")
	}
	if !target.IsAbs() {
		target = origin.ResolveReference(target)
	}
	if !strings.EqualFold(target.Scheme, "https") || !sameURLOrigin(origin, target) {
		return nil, errors.New("transaction tax download URL is outside the App Store Connect web origin")
	}
	if c == nil || c.httpClient == nil {
		return nil, errors.New("transaction tax web client is not configured")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, errors.New("transaction tax download request could not be created")
	}
	request.Header.Set("Accept", "application/octet-stream, application/zip, */*")
	request.Header.Set("Origin", origin.String())
	request.Header.Set("Referer", origin.String()+"/itc/payments_and_financial_reports/")
	request.Header.Set("X-Requested-With", "XMLHttpRequest")
	client := transactionTaxNoRetryHTTPClient(c.httpClient)
	// A bounded workflow context governs archive streaming, including the body.
	// Lower-level callers without a deadline retain their configured timeout.
	if _, bounded := ctx.Deadline(); bounded {
		client.Timeout = 0
	}

	previousCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(redirect *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("transaction tax download redirect limit reached")
		}
		if !strings.EqualFold(redirect.URL.Scheme, "https") || !sameURLOrigin(origin, redirect.URL) {
			return errors.New("transaction tax download redirect leaves the App Store Connect web origin")
		}
		if previousCheckRedirect != nil {
			return previousCheckRedirect(redirect, via)
		}
		return nil
	}
	if err := c.waitForRateLimit(ctx); err != nil {
		return nil, transactionTaxUnknownError("download", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, transactionTaxUnknownError("download", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_ = response.Body.Close()
		return nil, fmt.Errorf("transaction tax download failed with HTTP %d", response.StatusCode)
	}
	contentType := strings.TrimSpace(response.Header.Get("Content-Type"))
	if strings.HasPrefix(strings.ToLower(contentType), "text/html") {
		_ = response.Body.Close()
		return nil, errors.New("transaction tax download returned HTML instead of a report archive")
	}
	return &TransactionTaxReportDownload{
		Body:                      response.Body,
		ContentType:               contentType,
		ContentDispositionPresent: strings.TrimSpace(response.Header.Get("Content-Disposition")) != "",
	}, nil
}

func decodeTransactionTaxLocalization(body []byte) (map[string]string, error) {
	object, err := decodeTransactionTaxObject(body)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(object))
	for key, raw := range object {
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			result[key] = value
		}
	}
	return result, nil
}

func decodeTransactionTaxVendors(body []byte) ([]transactionTaxVendor, error) {
	value, err := decodeTransactionTaxDataValue(body)
	if err != nil {
		return nil, err
	}
	var rawVendors []map[string]json.RawMessage
	if err := json.Unmarshal(value, &rawVendors); err != nil {
		return nil, err
	}
	vendors := make([]transactionTaxVendor, 0, len(rawVendors))
	for _, raw := range rawVendors {
		id, err := transactionTaxIntegerField(raw, "sapVendorNumber")
		if err != nil || id <= 0 {
			return nil, errors.New("invalid SAP vendor number")
		}
		lastSold, err := transactionTaxNumberField(raw, "lastSoldUnit", true)
		if err != nil {
			return nil, err
		}
		arcade, err := transactionTaxBoolField(raw, "isArcadeVendor", false)
		if err != nil {
			return nil, err
		}
		vendors = append(vendors, transactionTaxVendor{SAPVendorNumber: id, LastSoldUnit: lastSold, IsArcadeVendor: arcade})
	}
	return vendors, nil
}

func selectTransactionTaxVendor(vendors []transactionTaxVendor) (transactionTaxVendor, error) {
	if len(vendors) == 0 {
		return transactionTaxVendor{}, errors.New("transaction tax provider has no SAP vendor")
	}
	selected := vendors[0]
	for _, vendor := range vendors[1:] {
		if vendor.LastSoldUnit > selected.LastSoldUnit {
			selected = vendor
		}
	}
	return selected, nil
}

func decodeTransactionTaxMonth(body []byte) (transactionTaxMonth, error) {
	object, err := decodeTransactionTaxObject(body)
	if err != nil {
		return transactionTaxMonth{}, err
	}
	hasVendorTax, err := transactionTaxBoolField(object, "hasVendorTaxReport", true)
	if err != nil {
		return transactionTaxMonth{}, err
	}
	arcade, err := transactionTaxBoolField(object, "isArcadeVendor", false)
	if err != nil {
		return transactionTaxMonth{}, err
	}
	var summaries []transactionTaxReportSummary
	if raw, ok := object["reportSummaries"]; ok {
		var decoded []struct {
			Proceeds []struct {
				RegionCurrency *struct {
					RegionCurrencyID json.RawMessage `json:"regionCurrencyId"`
					RegionCode       string          `json:"regionCode"`
					RegionNameKey    string          `json:"regionNameKey"`
					RegionName       string          `json:"regionName"`
				} `json:"regionCurrency"`
				FinancialReportType string          `json:"financialReportType"`
				Earned              json.RawMessage `json:"earned"`
			} `json:"proceedsByRegion"`
		}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return transactionTaxMonth{}, err
		}
		summaries = make([]transactionTaxReportSummary, 0, len(decoded))
		for _, summary := range decoded {
			converted := transactionTaxReportSummary{Proceeds: make([]transactionTaxProceed, 0, len(summary.Proceeds))}
			for _, proceed := range summary.Proceeds {
				var region *transactionTaxRegionCurrency
				if proceed.RegionCurrency != nil {
					id, err := transactionTaxIntegerRaw(proceed.RegionCurrency.RegionCurrencyID)
					if err != nil {
						return transactionTaxMonth{}, err
					}
					region = &transactionTaxRegionCurrency{
						ID:            id,
						RegionCode:    proceed.RegionCurrency.RegionCode,
						RegionNameKey: proceed.RegionCurrency.RegionNameKey,
						RegionName:    proceed.RegionCurrency.RegionName,
					}
				}
				converted.Proceeds = append(converted.Proceeds, transactionTaxProceed{
					RegionCurrency:      region,
					FinancialReportType: proceed.FinancialReportType,
					Earned:              proceed.Earned,
				})
			}
			summaries = append(summaries, converted)
		}
	}
	return transactionTaxMonth{HasVendorTaxReport: hasVendorTax, IsArcadeVendor: arcade, Summaries: summaries}, nil
}

func deriveTransactionTaxRegionIDs(month transactionTaxMonth, localization map[string]string) ([]int64, error) {
	seenKeys := make(map[string]struct{})
	groups := make([]transactionTaxRegionGroup, 0)
	groupIndex := make(map[string]int)
	for _, summary := range month.Summaries {
		for _, proceed := range summary.Proceeds {
			region := proceed.RegionCurrency
			if region == nil {
				return nil, errors.New("transaction tax period response has a malformed region currency")
			}
			if region.RegionCode == "Z2" {
				continue
			}
			hasEarnings, err := transactionTaxHasEarnings(proceed.Earned)
			if err != nil {
				return nil, errors.New("transaction tax period response has malformed proceeds")
			}
			displayIfNoEarnings := proceed.FinancialReportType == "D"
			if !hasEarnings && !displayIfNoEarnings {
				continue
			}
			if region.ID <= 0 || strings.TrimSpace(region.RegionNameKey) == "" {
				return nil, errors.New("transaction tax period response has an incomplete region currency")
			}
			// Apple's captured formatRegions helper deduplicates by regionNameKey
			// before grouping localized labels. Using the currency ID instead
			// would select records that the finance page omits.
			if _, seen := seenKeys[region.RegionNameKey]; seen {
				continue
			}
			seenKeys[region.RegionNameKey] = struct{}{}
			localizedName := localization[region.RegionNameKey]
			if localizedName == "" {
				localizedName = "**" + region.RegionNameKey + "**"
			}
			if index, ok := groupIndex[localizedName]; ok {
				groups[index].IDs = append(groups[index].IDs, region.ID)
				continue
			}
			groupIndex[localizedName] = len(groups)
			groups = append(groups, transactionTaxRegionGroup{
				SortName: transactionTaxSortName(localizedName),
				IDs:      []int64{region.ID},
			})
		}
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].SortName < groups[j].SortName })
	var result []int64
	for _, group := range groups {
		result = append(result, group.IDs...)
	}
	if len(result) == 0 {
		return nil, errors.New("transaction tax period response has no usable region currencies")
	}
	return result, nil
}

func transactionTaxSortName(value string) string {
	decomposed := norm.NFD.String(value)
	var builder strings.Builder
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		builder.WriteRune(unicode.ToLower(r))
	}
	return builder.String()
}

func transactionTaxHasEarnings(raw json.RawMessage) (bool, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false, nil
	}
	value, err := transactionTaxNumberRaw(raw)
	if err != nil {
		return false, err
	}
	return value != 0, nil
}

func decodeTransactionTaxObject(body []byte) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(body, &object); err != nil || object == nil {
		return nil, errors.New("expected JSON object")
	}
	if raw, ok := object["data"]; ok {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(raw, &nested); err == nil && nested != nil {
			return nested, nil
		}
	}
	return object, nil
}

func decodeTransactionTaxDataValue(body []byte) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, errors.New("empty JSON response")
	}
	if trimmed[0] == '[' {
		return trimmed, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil || object == nil {
		return nil, errors.New("expected JSON response")
	}
	if raw, ok := object["data"]; ok {
		return raw, nil
	}
	return nil, errors.New("response missing data")
}

func transactionTaxStringField(object map[string]json.RawMessage, key string) (string, error) {
	raw, ok := object[key]
	if !ok {
		return "", errors.New("missing string field")
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return value, nil
}

func transactionTaxBoolField(object map[string]json.RawMessage, key string, required bool) (bool, error) {
	raw, ok := object[key]
	if !ok {
		if required {
			return false, errors.New("missing boolean field")
		}
		return false, nil
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, err
	}
	return value, nil
}

func transactionTaxIntegerField(object map[string]json.RawMessage, key string) (int64, error) {
	raw, ok := object[key]
	if !ok {
		return 0, errors.New("missing integer field")
	}
	return transactionTaxIntegerRaw(raw)
}

func transactionTaxStrictIntegerField(object map[string]json.RawMessage, key string) (int64, error) {
	raw, ok := object[key]
	if !ok {
		return 0, errors.New("missing integer field")
	}
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" || strings.HasPrefix(value, `"`) {
		return 0, errors.New("invalid integer value")
	}
	return strconv.ParseInt(value, 10, 64)
}

func transactionTaxIntegerRaw(raw json.RawMessage) (int64, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return 0, errors.New("missing integer value")
	}
	if strings.HasPrefix(value, `"`) {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
		value = strings.TrimSpace(text)
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func transactionTaxNumberField(object map[string]json.RawMessage, key string, optional bool) (float64, error) {
	raw, ok := object[key]
	if !ok {
		if optional {
			return 0, nil
		}
		return 0, errors.New("missing number field")
	}
	return transactionTaxNumberRaw(raw)
}

func transactionTaxNumberRaw(raw json.RawMessage) (float64, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return 0, nil
	}
	if strings.HasPrefix(value, `"`) {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
		value = strings.TrimSpace(text)
	}
	if value == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}

func transactionTaxOperationError(operation string, err error) error {
	if err == nil {
		return fmt.Errorf("transaction tax %s failed", operation)
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		// Retain status-based auth guidance without exposing finance response
		// bodies, request identifiers, or provider correlation values.
		return fmt.Errorf("transaction tax %s failed: %w", operation, &APIError{Status: apiErr.Status})
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return transactionTaxUnknownError(operation, err)
	}
	return fmt.Errorf("transaction tax %s failed", operation)
}

func transactionTaxProtocolError(operation string) error {
	return fmt.Errorf("transaction tax %s was malformed", operation)
}

func transactionTaxUnknownError(operation string, err error) error {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return fmt.Errorf("transaction tax %s outcome is unknown after HTTP %d; no automatic retry was attempted", operation, apiErr.Status)
	}
	return fmt.Errorf("transaction tax %s outcome is unknown; no automatic retry was attempted", operation)
}
