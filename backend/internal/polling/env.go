package polling

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/logreport"
)

const (
	defaultP2PQuakeURL   = "https://api.p2pquake.net/v2/history?codes=551&limit=10"
	defaultJMAQuakeURL   = "https://www.jma.go.jp/bosai/quake/data/list.json"
	defaultUSGSQuakeURL  = "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/all_hour.geojson"
	defaultLidenTimesURL = "https://www.jma.go.jp/bosai/jmatile/data/nowc/targetTimes_N3.json"
	defaultLidenDataFmt  = "https://www.jma.go.jp/bosai/jmatile/data/nowc/%s/none/%s/surf/liden/data.geojson?id=liden"
	defaultNOAAXrayURL   = "https://services.swpc.noaa.gov/json/goes/primary/xrays-6-hour.json"
	defaultNOAAKpURL     = "https://services.swpc.noaa.gov/json/planetary_k_index_1m.json"
	defaultOpenMeteoURL  = "https://api.open-meteo.com/v1/forecast"
)

// EnvPoller polls open environmental, disaster, and space weather APIs
// (mode: "quake", "liden", "space", "weather").
type EnvPoller struct {
	store datastore.DataStore
}

// NewEnvPoller creates an initialized EnvPoller.
func NewEnvPoller(store datastore.DataStore) *EnvPoller {
	return &EnvPoller{store: store}
}

// isJapanCoord reports whether (lat, lon) falls within the Japan Meteorological Agency coverage box.
func isJapanCoord(lat, lon float64) bool {
	return lat >= 20.0 && lat <= 46.0 && lon >= 122.0 && lon <= 154.0
}

// Poll executes an environmental/disaster/space weather check according to pe.Mode.
func (ep *EnvPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if pe == nil {
		return &Result{
			State:   StateUnknown,
			Message: "polling entity is nil",
			Fields:  map[string]interface{}{"error": "polling entity is nil"},
		}, nil
	}
	mode := strings.ToLower(strings.TrimSpace(pe.Mode))
	switch mode {
	case "quake", "earthquake":
		return ep.pollQuake(ctx, pe, node)
	case "liden", "lightning":
		return ep.pollLiden(ctx, pe, node)
	case "space", "solar", "swpc":
		return ep.pollSpace(ctx, pe, node)
	case "weather", "openmeteo", "":
		return ep.pollWeather(ctx, pe, node)
	default:
		msg := fmt.Sprintf("unsupported env mode: %s", pe.Mode)
		return &Result{
			State:   StateUnknown,
			Message: msg,
			Fields:  map[string]interface{}{"error": msg},
		}, nil
	}
}

func envErrorResult(err error) *Result {
	msg := err.Error()
	return &Result{
		State:   StateUnknown,
		Message: msg,
		Fields:  map[string]interface{}{"error": msg},
	}
}

func (ep *EnvPoller) evaluateEnvScript(
	ctx context.Context,
	pe *datastore.PollingEnt,
	node *datastore.NodeEnt,
	mode, location, summary string,
	rttNano int64,
	fields map[string]interface{},
	sample logreport.EarthEnvDataEnt,
	syncEnvMonitor bool,
	defaultCheck func() (string, bool),
) *Result {
	newState := StateNormal
	if strings.TrimSpace(pe.Script) == "" {
		lvl, ok := defaultCheck()
		if !ok {
			if lvl != "" {
				newState = lvl
			} else if pe.Level != "" && pe.Level != "off" {
				newState = pe.Level
			} else {
				newState = StateWarn
			}
		}
	} else {
		vm := otto.New()
		SetupOttoVM(pe, vm, fields)
		for k, v := range fields {
			if !strings.HasPrefix(k, "_") {
				_ = vm.Set(k, v)
			}
		}
		_ = vm.Set("interval", pe.PollInt)
		val, err := vm.Run(pe.Script)
		if err != nil {
			return envErrorResult(fmt.Errorf("invalid script err=%v", err))
		}
		if lvl, ok := fields["_level"].(string); ok && lvl != "" {
			newState = lvl
			delete(fields, "_level")
		} else if ok, _ := val.ToBoolean(); !ok {
			newState = pe.Level
			if newState == "" || newState == "off" {
				newState = StateWarn
			}
		}
	}

	if summary != "" {
		fields["_summary"] = summary
	}

	// Record to sensor monitor & logreport EarthEnv / EnvMonitor
	hostName := "TWSNMP"
	if node != nil && node.Name != "" {
		hostName = node.Name
	} else if ep.store != nil && pe.NodeID != "" {
		if n, err := ep.store.GetNode(ctx, pe.NodeID); err == nil && n != nil && n.Name != "" {
			hostName = n.Name
		}
	}
	if ep.store != nil {
		ep.store.UpdateSensor(hostName, "env-"+mode, pe.Name, 1)
		sample.Time = time.Now().UnixNano()
		sample.State = newState
		_ = logreport.RecordEarthEnvSample(
			ctx,
			ep.store,
			pe.ID,
			pe.NodeID,
			hostName,
			mode,
			pe.Name,
			location,
			summary,
			newState,
			sample,
			syncEnvMonitor,
		)
	}

	return &Result{
		State:   newState,
		RTT:     time.Duration(rttNano),
		Message: summary,
		Fields:  fields,
	}
}

// ============================================================================
// 1. Earthquake Monitor (Mode: "quake") - P2P地震情報 API v2 / JMA / USGS
// ============================================================================

type p2pQuakeRecord struct {
	ID         string `json:"id"`
	Code       int    `json:"code"`
	Time       string `json:"time"`
	Earthquake struct {
		Time       string `json:"time"`
		MaxScale   int    `json:"maxScale"`
		Hypocenter struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Depth     float64 `json:"depth"`
			Magnitude float64 `json:"magnitude"`
		} `json:"hypocenter"`
	} `json:"earthquake"`
	Points []struct {
		Pref  string `json:"pref"`
		Addr  string `json:"addr"`
		Scale int    `json:"scale"`
	} `json:"points"`
}

type jmaQuakeRecord struct {
	EventID string `json:"eid"`
	At      string `json:"at"`
	An      string `json:"anm"`
	Mag     string `json:"mag"`
	MaxI    string `json:"maxi"`
	Cod     string `json:"cod"` // ISO6709 e.g. "+35.6+140.1-50000/"
	Int     []struct {
		Code string `json:"code"`
		MaxI string `json:"maxi"`
	} `json:"int"`
}

type usgsQuakeFeed struct {
	Type     string `json:"type"`
	Features []struct {
		ID         string `json:"id"`
		Properties struct {
			Mag     *float64 `json:"mag"`
			Place   string   `json:"place"`
			Time    int64    `json:"time"` // milliseconds since epoch
			MMI     *float64 `json:"mmi"`
			Tsunami int      `json:"tsunami"`
		} `json:"properties"`
		Geometry struct {
			Coordinates []float64 `json:"coordinates"` // [lon, lat, depthKm]
		} `json:"geometry"`
	} `json:"features"`
}

func mmiToJMAIntensity(mmi float64) float64 {
	switch {
	case mmi >= 9.5:
		return 7.0
	case mmi >= 8.5:
		return 6.5
	case mmi >= 7.5:
		return 6.0
	case mmi >= 6.5:
		return 5.5
	case mmi >= 5.5:
		return 5.0
	case mmi >= 4.5:
		return 4.0
	case mmi >= 3.5:
		return 3.0
	case mmi >= 2.5:
		return 2.0
	case mmi >= 1.5:
		return 1.0
	default:
		return 0.0
	}
}

func (ep *EnvPoller) pollQuake(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	pMap := parseEnvParams(pe.Params)
	targetLat, targetLon := resolveTargetLatLon(ctx, ep.store, pe, node, pMap)
	source := strings.ToLower(pMap["source"])
	if source == "" || source == "auto" {
		if isJapanCoord(targetLat, targetLon) {
			source = "p2p"
		} else {
			source = "usgs"
		}
	}
	prefFilter := pMap["pref"]
	areaFilter := pMap["area"]
	maxAgeSec := 3600.0
	if v, err := strconv.ParseFloat(pMap["max_age"], 64); err == nil && v > 0 {
		maxAgeSec = v
	}
	radiusKm := 300.0
	if v, err := strconv.ParseFloat(pMap["radius"], 64); err == nil && v > 0 {
		radiusKm = v
	}

	apiURL := pMap["url"]
	cacheKey := "quake:" + source
	if apiURL != "" {
		cacheKey = "quake:custom:" + apiURL
	} else if source == "jma" {
		apiURL = defaultJMAQuakeURL
	} else if source == "usgs" {
		apiURL = defaultUSGSQuakeURL
	} else {
		apiURL = defaultP2PQuakeURL
	}

	body, fromCache, rtt, err := globalEnvCache.fetchWithCache(ctx, cacheKey, apiURL, 60*time.Second, pe.Timeout)
	if err != nil {
		return envErrorResult(err), nil
	}

	var (
		intensity    float64
		maxIntensity float64
		mmiVal       float64
		tsunamiFlag  int
		magnitude    float64
		depthKm      float64
		distanceKm   = 9999.0
		elapsedSec   float64
		quakeTimeStr string
		hypocenter   string
	)

	now := time.Now()
	switch source {
	case "usgs":
		var feed usgsQuakeFeed
		if err := json.Unmarshal(body, &feed); err != nil {
			return envErrorResult(fmt.Errorf("parse usgs geojson err=%v", err)), nil
		}
		bestScore := -1.0
		for _, f := range feed.Features {
			if len(f.Geometry.Coordinates) < 2 {
				continue
			}
			qLon := f.Geometry.Coordinates[0]
			qLat := f.Geometry.Coordinates[1]
			var qDepth float64
			if len(f.Geometry.Coordinates) >= 3 {
				qDepth = math.Abs(f.Geometry.Coordinates[2])
			}
			dist := calcDistanceKm(targetLat, targetLon, qLat, qLon)
			if dist > radiusKm {
				continue
			}
			qTime := time.UnixMilli(f.Properties.Time)
			elSec := now.Sub(qTime).Seconds()
			if elSec < 0 {
				elSec = 0
			}
			if elSec > maxAgeSec {
				continue
			}
			var mag float64
			if f.Properties.Mag != nil && *f.Properties.Mag > 0 {
				mag = *f.Properties.Mag
			}
			var mmi float64
			if f.Properties.MMI != nil && *f.Properties.MMI > 0 {
				mmi = *f.Properties.MMI
			}
			score := mmi*10.0 + mag
			if score > bestScore {
				bestScore = score
				mmiVal = math.Round(mmi*10) / 10
				tsunamiFlag = f.Properties.Tsunami
				magnitude = math.Round(mag*10) / 10
				depthKm = math.Round(qDepth*10) / 10
				distanceKm = dist
				elapsedSec = elSec
				quakeTimeStr = qTime.Format(time.RFC3339)
				hypocenter = f.Properties.Place
				intensity = mmiToJMAIntensity(mmi)
				maxIntensity = intensity
			}
		}
	case "jma":
		var list []jmaQuakeRecord
		if err := json.Unmarshal(body, &list); err != nil {
			return envErrorResult(fmt.Errorf("parse jma json err=%v", err)), nil
		}
		if len(list) > 0 {
			latest := list[0]
			qTime := parseQuakeTime(latest.At, now)
			elapsedSec = now.Sub(qTime).Seconds()
			if elapsedSec < 0 {
				elapsedSec = 0
			}
			quakeTimeStr = qTime.Format(time.RFC3339)
			hypocenter = latest.An
			magnitude, _ = strconv.ParseFloat(latest.Mag, 64)
			if magnitude < 0 {
				magnitude = 0
			}
			maxIntensity = jmaScaleStrToFloat(latest.MaxI)
			if qLat, qLon, qDepth, ok := parseISO6709(latest.Cod); ok {
				depthKm = qDepth
				distanceKm = calcDistanceKm(targetLat, targetLon, qLat, qLon)
			}
			if elapsedSec <= maxAgeSec {
				if areaFilter != "" {
					for _, it := range latest.Int {
						if strings.Contains(it.Code, areaFilter) {
							s := jmaScaleStrToFloat(it.MaxI)
							if s > intensity {
								intensity = s
							}
						}
					}
				} else {
					intensity = maxIntensity
				}
			}
		}
	default:
		var list []p2pQuakeRecord
		if err := json.Unmarshal(body, &list); err != nil {
			return envErrorResult(fmt.Errorf("parse p2pquake json err=%v", err)), nil
		}
		if len(list) > 0 {
			latest := list[0]
			tRaw := latest.Earthquake.Time
			if tRaw == "" {
				tRaw = latest.Time
			}
			qTime := parseQuakeTime(tRaw, now)
			elapsedSec = now.Sub(qTime).Seconds()
			if elapsedSec < 0 {
				elapsedSec = 0
			}
			quakeTimeStr = qTime.Format(time.RFC3339)
			hypocenter = latest.Earthquake.Hypocenter.Name
			magnitude = latest.Earthquake.Hypocenter.Magnitude
			if magnitude < 0 {
				magnitude = 0
			}
			depthKm = latest.Earthquake.Hypocenter.Depth
			if depthKm < 0 {
				depthKm = 0
			}
			maxIntensity = p2pScaleToFloat(latest.Earthquake.MaxScale)
			hLat := latest.Earthquake.Hypocenter.Latitude
			hLon := latest.Earthquake.Hypocenter.Longitude
			if hLat >= -90 && hLat <= 90 && hLon >= -180 && hLon <= 180 && (hLat != 0 || hLon != 0) {
				distanceKm = calcDistanceKm(targetLat, targetLon, hLat, hLon)
			}

			if elapsedSec <= maxAgeSec {
				matchedPoint := false
				if prefFilter != "" || areaFilter != "" {
					for _, pt := range latest.Points {
						if prefFilter != "" && !strings.Contains(pt.Pref, prefFilter) {
							continue
						}
						if areaFilter != "" && !strings.Contains(pt.Addr, areaFilter) && !strings.Contains(pt.Pref, areaFilter) {
							continue
						}
						matchedPoint = true
						s := p2pScaleToFloat(pt.Scale)
						if s > intensity {
							intensity = s
						}
					}
				}
				if !matchedPoint && prefFilter == "" && areaFilter == "" {
					if len(latest.Points) > 0 {
						for _, pt := range latest.Points {
							s := p2pScaleToFloat(pt.Scale)
							if s > intensity {
								intensity = s
							}
						}
					} else {
						intensity = maxIntensity
					}
				}
			}
		}
	}

	distanceKm = math.Round(distanceKm*10) / 10
	elapsedSec = math.Round(elapsedSec)

	fields := map[string]interface{}{
		"intensity":     intensity,
		"max_intensity": maxIntensity,
		"mmi":           mmiVal,
		"tsunami":       float64(tsunamiFlag),
		"magnitude":     magnitude,
		"depth_km":      depthKm,
		"distance_km":   distanceKm,
		"elapsed_sec":   elapsedSec,
		"quake_time":    quakeTimeStr,
		"hypocenter":    hypocenter,
		"source":        source,
		"cached":        boolToFloat(fromCache),
	}

	locLabel := fmt.Sprintf("%.4f,%.4f", targetLat, targetLon)
	if prefFilter != "" {
		locLabel = prefFilter
	} else if areaFilter != "" {
		locLabel = areaFilter
	}
	summary := fmt.Sprintf("震央:%s M%.1f 最大震度:%.1f (対象震度:%.1f, 距離:%.1fkm)",
		orDefault(hypocenter, "-"), magnitude, maxIntensity, intensity, distanceKm)

	sample := logreport.EarthEnvDataEnt{
		Intensity:    intensity,
		MaxIntensity: maxIntensity,
		Magnitude:    magnitude,
		DepthKm:      depthKm,
		DistanceKm:   distanceKm,
		Hypocenter:   hypocenter,
		QuakeTime:    quakeTimeStr,
	}

	return ep.evaluateEnvScript(ctx, pe, node, "quake", locLabel, summary, rtt, fields, sample, false, func() (string, bool) {
		if intensity >= 5.0 {
			return StateHigh, false
		}
		if intensity >= 3.0 {
			return StateWarn, false
		}
		return StateNormal, true
	}), nil
}

func p2pScaleToFloat(scale int) float64 {
	switch scale {
	case 10:
		return 1.0
	case 20:
		return 2.0
	case 30:
		return 3.0
	case 40:
		return 4.0
	case 45, 46:
		return 5.0 // 5弱
	case 50:
		return 5.5 // 5強
	case 55:
		return 6.0 // 6弱
	case 60:
		return 6.5 // 6強
	case 70:
		return 7.0
	default:
		return 0.0
	}
}

func jmaScaleStrToFloat(s string) float64 {
	switch strings.TrimSpace(s) {
	case "1":
		return 1.0
	case "2":
		return 2.0
	case "3":
		return 3.0
	case "4":
		return 4.0
	case "5-", "5弱":
		return 5.0
	case "5+", "5強":
		return 5.5
	case "6-", "6弱":
		return 6.0
	case "6+", "6強":
		return 6.5
	case "7":
		return 7.0
	default:
		return 0.0
	}
}

var jstZone = time.FixedZone("JST", 9*3600)

func parseQuakeTime(s string, def time.Time) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	for _, layout := range []string{
		"2006/01/02 15:04:05",
		"2006/01/02 15:04",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
	} {
		if t, err := time.ParseInLocation(layout, s, jstZone); err == nil {
			return t
		}
	}
	return def
}

// parseISO6709 parses JMA coordinate string like "+35.6+140.1-50000/" into (lat, lon, depthKm, ok).
func parseISO6709(cod string) (float64, float64, float64, bool) {
	cod = strings.TrimSpace(strings.TrimSuffix(cod, "/"))
	if len(cod) < 6 {
		return 0, 0, 0, false
	}
	var parts []string
	start := 0
	for i := 1; i < len(cod); i++ {
		if cod[i] == '+' || cod[i] == '-' {
			parts = append(parts, cod[start:i])
			start = i
		}
	}
	parts = append(parts, cod[start:])
	if len(parts) < 2 {
		return 0, 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(parts[0], 64)
	lon, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil {
		return 0, 0, 0, false
	}
	var depthKm float64
	if len(parts) >= 3 {
		if dMeters, err3 := strconv.ParseFloat(parts[2], 64); err3 == nil {
			depthKm = math.Abs(dMeters) / 1000.0
		}
	}
	return lat, lon, depthKm, true
}

// ============================================================================
// 2. Lightning Monitor (Mode: "liden") - JMA LIDEN GeoJSON / Open-Meteo
// ============================================================================

type jmaTargetTime struct {
	Basetime  string   `json:"basetime"`
	Validtime string   `json:"validtime"`
	Elements  []string `json:"elements"`
}

type lidenGeoJSON struct {
	Type     string `json:"type"`
	Features []struct {
		Type     string `json:"type"`
		Geometry struct {
			Type        string    `json:"type"`
			Coordinates []float64 `json:"coordinates"` // [lon, lat]
		} `json:"geometry"`
		Properties map[string]any `json:"properties"`
	} `json:"features"`
}

func (ep *EnvPoller) pollLiden(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	pMap := parseEnvParams(pe.Params)
	targetLat, targetLon := resolveTargetLatLon(ctx, ep.store, pe, node, pMap)

	source := strings.ToLower(pMap["source"])
	if source == "" {
		source = "auto"
	}
	if pMap["geojson_url"] == "" && (source == "openmeteo" || (source == "auto" && !isJapanCoord(targetLat, targetLon))) {
		return ep.pollLidenOpenMeteo(ctx, pe, node, pMap, targetLat, targetLon)
	}

	radiusKm := 20.0
	if v, err := strconv.ParseFloat(pMap["radius"], 64); err == nil && v > 0 {
		radiusKm = v
	}
	warnRadiusKm := 10.0
	if v, err := strconv.ParseFloat(pMap["warn_radius"], 64); err == nil && v > 0 {
		warnRadiusKm = v
	}

	var (
		geoBody   []byte
		fromCache bool
		totalRTT  int64
		obsTime   string
	)

	if directGeoURL := pMap["geojson_url"]; directGeoURL != "" {
		var err error
		geoBody, fromCache, totalRTT, err = globalEnvCache.fetchWithCache(ctx, "liden:direct:"+directGeoURL, directGeoURL, 60*time.Second, pe.Timeout)
		if err != nil {
			return envErrorResult(err), nil
		}
		obsTime = time.Now().Format(time.RFC3339)
	} else {
		timesURL := pMap["url"]
		if timesURL == "" {
			timesURL = defaultLidenTimesURL
		}
		timesBody, cached1, rtt1, err := globalEnvCache.fetchWithCache(ctx, "liden:times:"+timesURL, timesURL, 60*time.Second, pe.Timeout)
		if err != nil {
			return envErrorResult(err), nil
		}
		totalRTT += rtt1
		fromCache = cached1

		var targetTimes []jmaTargetTime
		if err := json.Unmarshal(timesBody, &targetTimes); err != nil {
			return envErrorResult(fmt.Errorf("parse targetTimes err=%v", err)), nil
		}
		if len(targetTimes) == 0 {
			return envErrorResult(fmt.Errorf("no targetTimes available")), nil
		}
		var latest jmaTargetTime
		foundLiden := false
		for _, tt := range targetTimes {
			if tt.Basetime == "" || tt.Validtime == "" {
				continue
			}
			for _, el := range tt.Elements {
				if el == "liden" {
					latest = tt
					foundLiden = true
					break
				}
			}
			if foundLiden {
				break
			}
		}
		if !foundLiden {
			latest = targetTimes[0]
		}
		if latest.Basetime == "" || latest.Validtime == "" {
			return envErrorResult(fmt.Errorf("no valid liden targetTime available")), nil
		}
		if t, err := time.Parse("20060102150405", latest.Validtime); err == nil {
			obsTime = t.In(jstZone).Format(time.RFC3339)
		} else {
			obsTime = latest.Validtime
		}

		dataURL := fmt.Sprintf(defaultLidenDataFmt, latest.Basetime, latest.Validtime)
		if baseFmt := pMap["data_url_fmt"]; baseFmt != "" {
			dataURL = fmt.Sprintf(baseFmt, latest.Basetime, latest.Validtime)
		}
		var cached2 bool
		var rtt2 int64
		geoBody, cached2, rtt2, err = globalEnvCache.fetchWithCache(ctx, "liden:data:"+latest.Validtime, dataURL, 60*time.Second, pe.Timeout)
		if err != nil {
			return envErrorResult(err), nil
		}
		totalRTT += rtt2
		fromCache = fromCache && cached2
	}

	var geo lidenGeoJSON
	if err := json.Unmarshal(geoBody, &geo); err != nil {
		return envErrorResult(fmt.Errorf("parse liden geojson err=%v", err)), nil
	}

	var (
		strikeCount   int
		closeCount    int
		cgCount       int
		icCount       int
		nationalCount int
		nearestKm     = 9999.0
	)

	for _, f := range geo.Features {
		coords := f.Geometry.Coordinates
		if len(coords) < 2 {
			continue
		}
		sLon := coords[0]
		sLat := coords[1]
		nationalCount++

		dist := calcDistanceKm(targetLat, targetLon, sLat, sLon)
		if dist < nearestKm {
			nearestKm = dist
		}
		if dist <= warnRadiusKm {
			closeCount++
		}
		if dist <= radiusKm {
			strikeCount++
			isIC := false
			if f.Properties != nil {
				if tVal, ok := f.Properties["type"]; ok {
					switch tv := tVal.(type) {
					case float64:
						// JMA LIDEN discharge type: 0 = IC (cloud), 1+ = CG (ground)
						isIC = int(tv) == 0
					case string:
						isIC = strings.EqualFold(tv, "ic") || strings.EqualFold(tv, "cloud")
					}
				}
			}
			if isIC {
				icCount++
			} else {
				cgCount++
			}
		}
	}

	nearestKm = math.Round(nearestKm*10) / 10

	srcLabel := "jma"
	if pMap["geojson_url"] != "" {
		srcLabel = "custom"
	}

	fields := map[string]interface{}{
		"strike_count":   float64(strikeCount),
		"close_count":    float64(closeCount),
		"cg_count":       float64(cgCount),
		"ic_count":       float64(icCount),
		"nearest_km":     nearestKm,
		"national_count": float64(nationalCount),
		"source":         srcLabel,
		"obs_time":       obsTime,
		"cached":         boolToFloat(fromCache),
	}

	locLabel := fmt.Sprintf("%.4f,%.4f (r=%.0fkm)", targetLat, targetLon, radiusKm)
	summary := fmt.Sprintf("%.0fkm以内落雷:%d回 (%.0fkm以内:%d回, 最接近:%.1fkm)",
		warnRadiusKm, closeCount, radiusKm, strikeCount, nearestKm)

	sample := logreport.EarthEnvDataEnt{
		StrikeCount: strikeCount,
		CloseCount:  closeCount,
		CGCount:     cgCount,
		ICCount:     icCount,
		NearestKm:   nearestKm,
	}

	return ep.evaluateEnvScript(ctx, pe, node, "liden", locLabel, summary, totalRTT, fields, sample, false, func() (string, bool) {
		if closeCount > 0 {
			return StateHigh, false
		}
		if strikeCount > 0 {
			return StateWarn, false
		}
		return StateNormal, true
	}), nil
}

func (ep *EnvPoller) pollLidenOpenMeteo(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt, pMap map[string]string, targetLat, targetLon float64) (*Result, error) {
	baseURL := pMap["url"]
	if baseURL == "" {
		baseURL = defaultOpenMeteoURL
	}
	reqURL := fmt.Sprintf(
		"%s?latitude=%.4f&longitude=%.4f&current=temperature_2m,relative_humidity_2m,surface_pressure,pressure_msl,wind_speed_10m,wind_gusts_10m,precipitation,weather_code,cape&hourly=surface_pressure,cape&past_hours=6&forecast_hours=1&wind_speed_unit=ms",
		baseURL, targetLat, targetLon,
	)
	if strings.Contains(baseURL, "?") {
		reqURL = baseURL
	}
	cacheKey := fmt.Sprintf("weather:%.2f,%.2f:%s", targetLat, targetLon, baseURL)
	body, fromCache, rtt, err := globalEnvCache.fetchWithCache(ctx, cacheKey, reqURL, 900*time.Second, pe.Timeout)
	if err != nil {
		return envErrorResult(err), nil
	}

	var resp openMeteoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return envErrorResult(fmt.Errorf("parse open-meteo json err=%v", err)), nil
	}

	wCode := resp.Current.WeatherCode
	cape := resp.Current.CAPE
	if cape == 0 && len(resp.Hourly.CAPE) > 0 {
		cape = resp.Hourly.CAPE[len(resp.Hourly.CAPE)-1]
	}

	strikeCount := 0
	closeCount := 0
	nearestKm := 9999.0

	if wCode >= 95 {
		strikeCount = 1
		nearestKm = 0.0
		if wCode >= 96 || cape >= 2000 {
			closeCount = 1
		}
	} else if cape >= 1500 {
		strikeCount = 1
		nearestKm = 15.0
	}

	fields := map[string]interface{}{
		"strike_count":   float64(strikeCount),
		"close_count":    float64(closeCount),
		"cg_count":       float64(strikeCount),
		"ic_count":       0.0,
		"nearest_km":     nearestKm,
		"national_count": 0.0,
		"cape":           cape,
		"weather_code":   float64(wCode),
		"source":         "openmeteo",
		"obs_time":       resp.Current.Time,
		"cached":         boolToFloat(fromCache),
	}

	locLabel := fmt.Sprintf("%.4f,%.4f (Global/Open-Meteo)", targetLat, targetLon)
	summary := fmt.Sprintf("雷雨リスク(Open-Meteo): WMOコード=%d CAPE=%.0fJ/kg (発雷判定=%d, 強雷=%d)",
		wCode, cape, strikeCount, closeCount)

	sample := logreport.EarthEnvDataEnt{
		StrikeCount: strikeCount,
		CloseCount:  closeCount,
		CGCount:     strikeCount,
		NearestKm:   nearestKm,
		CAPE:        cape,
		WeatherCode: wCode,
	}

	return ep.evaluateEnvScript(ctx, pe, node, "liden", locLabel, summary, rtt, fields, sample, false, func() (string, bool) {
		if closeCount > 0 {
			return StateHigh, false
		}
		if strikeCount > 0 {
			return StateWarn, false
		}
		return StateNormal, true
	}), nil
}

// ============================================================================
// 3. Space Weather Monitor (Mode: "space") - NOAA SWPC GOES X-ray & Kp Index
// ============================================================================

type noaaXrayRecord struct {
	TimeTag string  `json:"time_tag"`
	Flux    float64 `json:"flux"`
	Energy  string  `json:"energy"`
}

type noaaKpRecord struct {
	TimeTag     string  `json:"time_tag"`
	KpIndex     float64 `json:"kp_index"`
	EstimatedKp float64 `json:"estimated_kp"`
}

func (ep *EnvPoller) pollSpace(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	pMap := parseEnvParams(pe.Params)
	xrayURL := pMap["xray_url"]
	if xrayURL == "" {
		xrayURL = defaultNOAAXrayURL
	}
	kpURL := pMap["kp_url"]
	if kpURL == "" {
		kpURL = defaultNOAAKpURL
	}

	var (
		xrayFlux   float64
		flareScore float64
		flareClass = "A0.0"
		kpIndex    float64
		gScale     int
		totalRTT   int64
		fromCache  = true
	)

	target := strings.ToLower(pMap["target"])
	fetchXray := target == "" || target == "all" || target == "xray"
	fetchKp := target == "" || target == "all" || target == "kp"

	if fetchXray {
		xBody, cached, rtt, err := globalEnvCache.fetchWithCache(ctx, "space:xray:"+xrayURL, xrayURL, 180*time.Second, pe.Timeout)
		if err != nil {
			return envErrorResult(fmt.Errorf("goes xray err=%v", err)), nil
		}
		totalRTT += rtt
		fromCache = fromCache && cached

		var xList []noaaXrayRecord
		if err := json.Unmarshal(xBody, &xList); err != nil {
			return envErrorResult(fmt.Errorf("parse goes xray err=%v", err)), nil
		}
		for i := len(xList) - 1; i >= 0; i-- {
			rec := xList[i]
			if (rec.Energy == "" || rec.Energy == "0.1-0.8nm") && rec.Flux > 0 {
				xrayFlux = rec.Flux
				break
			}
		}
		if xrayFlux == 0 && len(xList) > 0 {
			xrayFlux = xList[len(xList)-1].Flux
		}
		flareScore, flareClass = calcSolarFlareClass(xrayFlux)
	}

	if fetchKp {
		kBody, cached, rtt, err := globalEnvCache.fetchWithCache(ctx, "space:kp:"+kpURL, kpURL, 180*time.Second, pe.Timeout)
		if err != nil {
			return envErrorResult(fmt.Errorf("kp index err=%v", err)), nil
		}
		totalRTT += rtt
		fromCache = fromCache && cached

		var kList []noaaKpRecord
		if err := json.Unmarshal(kBody, &kList); err != nil {
			return envErrorResult(fmt.Errorf("parse kp index err=%v", err)), nil
		}
		if len(kList) > 0 {
			latest := kList[len(kList)-1]
			kpIndex = latest.EstimatedKp
			if kpIndex <= 0 {
				kpIndex = latest.KpIndex
			}
		}
		gScale = calcGeomagneticGScale(kpIndex)
	}

	flareScore = math.Round(flareScore*100) / 100
	kpIndex = math.Round(kpIndex*100) / 100

	fields := map[string]interface{}{
		"xray_flux":   xrayFlux,
		"flare_score": flareScore,
		"flare_class": flareClass,
		"kp_index":    kpIndex,
		"g_scale":     float64(gScale),
		"cached":      boolToFloat(fromCache),
	}

	summary := fmt.Sprintf("太陽フレア:%s (flux:%.2e W/m2), 地磁気Kp指数:%.1f (G%d)",
		flareClass, xrayFlux, kpIndex, gScale)

	sample := logreport.EarthEnvDataEnt{
		XrayFlux:   xrayFlux,
		FlareScore: flareScore,
		FlareClass: flareClass,
		KpIndex:    kpIndex,
		GScale:     gScale,
	}

	return ep.evaluateEnvScript(ctx, pe, node, "space", "Global (NOAA SWPC)", summary, totalRTT, fields, sample, false, func() (string, bool) {
		if flareScore >= 4.0 || kpIndex >= 7.0 {
			return StateHigh, false
		}
		if flareScore >= 3.0 || kpIndex >= 5.0 {
			return StateWarn, false
		}
		return StateNormal, true
	}), nil
}

// calcSolarFlareClass converts GOES X-ray flux (W/m^2) into logarithmic score and flare class (A, B, C, M, X).
func calcSolarFlareClass(flux float64) (float64, string) {
	if flux <= 0 {
		return 0, "A0.0"
	}
	score := math.Log10(flux) + 8.0
	if score < 0 {
		score = 0
	}
	switch {
	case flux >= 1e-4:
		return score, fmt.Sprintf("X%.1f", flux/1e-4)
	case flux >= 1e-5:
		return score, fmt.Sprintf("M%.1f", flux/1e-5)
	case flux >= 1e-6:
		return score, fmt.Sprintf("C%.1f", flux/1e-6)
	case flux >= 1e-7:
		return score, fmt.Sprintf("B%.1f", flux/1e-7)
	default:
		return score, fmt.Sprintf("A%.1f", flux/1e-8)
	}
}

func calcGeomagneticGScale(kp float64) int {
	switch {
	case kp >= 9.0:
		return 5
	case kp >= 8.0:
		return 4
	case kp >= 7.0:
		return 3
	case kp >= 6.0:
		return 2
	case kp >= 5.0:
		return 1
	default:
		return 0
	}
}

// ============================================================================
// 4. Weather / Pressure / CAPE Monitor (Mode: "weather") - Open-Meteo API
// ============================================================================

type openMeteoResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Current   struct {
		Time             string  `json:"time"`
		Temperature2m    float64 `json:"temperature_2m"`
		RelativeHumidity float64 `json:"relative_humidity_2m"`
		SurfacePressure  float64 `json:"surface_pressure"`
		PressureMSL      float64 `json:"pressure_msl"`
		WindSpeed10m     float64 `json:"wind_speed_10m"`
		WindGusts10m     float64 `json:"wind_gusts_10m"`
		Precipitation    float64 `json:"precipitation"`
		WeatherCode      int     `json:"weather_code"`
		CAPE             float64 `json:"cape"`
	} `json:"current"`
	Hourly struct {
		Time            []string  `json:"time"`
		SurfacePressure []float64 `json:"surface_pressure"`
		CAPE            []float64 `json:"cape"`
	} `json:"hourly"`
}

func (ep *EnvPoller) pollWeather(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	pMap := parseEnvParams(pe.Params)
	targetLat, targetLon := resolveTargetLatLon(ctx, ep.store, pe, node, pMap)

	baseURL := pMap["url"]
	if baseURL == "" {
		baseURL = defaultOpenMeteoURL
	}

	reqURL := fmt.Sprintf(
		"%s?latitude=%.4f&longitude=%.4f&current=temperature_2m,relative_humidity_2m,surface_pressure,pressure_msl,wind_speed_10m,wind_gusts_10m,precipitation,weather_code,cape&hourly=surface_pressure,cape&past_hours=6&forecast_hours=1&wind_speed_unit=ms",
		baseURL, targetLat, targetLon,
	)
	if strings.Contains(baseURL, "?") {
		reqURL = baseURL
	}

	// Grid cache key rounded to 2 decimal places (~1.1km mesh)
	cacheKey := fmt.Sprintf("weather:%.2f,%.2f:%s", targetLat, targetLon, baseURL)
	body, fromCache, rtt, err := globalEnvCache.fetchWithCache(ctx, cacheKey, reqURL, 900*time.Second, pe.Timeout)
	if err != nil {
		return envErrorResult(err), nil
	}

	var resp openMeteoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return envErrorResult(fmt.Errorf("parse open-meteo json err=%v", err)), nil
	}

	pressure := resp.Current.SurfacePressure
	if pressure == 0 && resp.Current.PressureMSL > 0 {
		pressure = resp.Current.PressureMSL
	}
	pressureMSL := resp.Current.PressureMSL
	if pressureMSL == 0 {
		pressureMSL = pressure
	}

	var pressureDiff3h float64
	nHourly := len(resp.Hourly.SurfacePressure)
	if nHourly >= 4 && pressure > 0 {
		curIdx := nHourly - 1
		if curIdx >= 3 {
			prev3h := resp.Hourly.SurfacePressure[curIdx-3]
			if prev3h > 0 {
				pressureDiff3h = math.Round((pressure-prev3h)*10) / 10
			}
		}
	}

	cape := resp.Current.CAPE
	if cape == 0 && len(resp.Hourly.CAPE) > 0 {
		cape = resp.Hourly.CAPE[len(resp.Hourly.CAPE)-1]
	}

	fields := map[string]interface{}{
		"temp":             resp.Current.Temperature2m,
		"humidity":         resp.Current.RelativeHumidity,
		"pressure":         pressure,
		"pressure_msl":     pressureMSL,
		"pressure_diff_3h": pressureDiff3h,
		"cape":             cape,
		"wind":             resp.Current.WindSpeed10m,
		"wind_gust":        resp.Current.WindGusts10m,
		"precipitation":    resp.Current.Precipitation,
		"weather_code":     float64(resp.Current.WeatherCode),
		"cached":           boolToFloat(fromCache),
	}

	syncEnvReport := true
	if strings.EqualFold(pMap["sync_env_report"], "false") || pMap["sync_env_report"] == "0" {
		syncEnvReport = false
	}

	locLabel := fmt.Sprintf("%.4f,%.4f", targetLat, targetLon)
	summary := fmt.Sprintf("気温:%.1f℃ 湿度:%.0f%% 気圧:%.1fhPa (3h変化:%+.1fhPa) CAPE:%.0fJ/kg 風速:%.1fm/s",
		resp.Current.Temperature2m, resp.Current.RelativeHumidity, pressure, pressureDiff3h, cape, resp.Current.WindSpeed10m)

	sample := logreport.EarthEnvDataEnt{
		Temp:           resp.Current.Temperature2m,
		Humidity:       resp.Current.RelativeHumidity,
		Pressure:       pressure,
		PressureMSL:    pressureMSL,
		PressureDiff3h: pressureDiff3h,
		CAPE:           cape,
		WindSpeed:      resp.Current.WindSpeed10m,
		WindGust:       resp.Current.WindGusts10m,
		Precipitation:  resp.Current.Precipitation,
		WeatherCode:    resp.Current.WeatherCode,
	}

	return ep.evaluateEnvScript(ctx, pe, node, "weather", locLabel, summary, rtt, fields, sample, syncEnvReport, func() (string, bool) {
		if resp.Current.WindGusts10m >= 25.0 || cape >= 2500 || pressureDiff3h <= -10.0 {
			return StateHigh, false
		}
		if resp.Current.WindGusts10m >= 15.0 || cape >= 1000 || pressureDiff3h <= -5.0 || resp.Current.WeatherCode >= 95 {
			return StateWarn, false
		}
		return StateNormal, true
	}), nil
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
