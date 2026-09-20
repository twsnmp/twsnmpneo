package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"go.etcd.io/bbolt"
)

// MigrationReport summarizes the results of importing legacy TWSNMP FC data.
type MigrationReport struct {
	SourceDB       string   `json:"sourceDB"`
	NodesImported  int      `json:"nodesImported"`
	LinesImported  int      `json:"linesImported"`
	NwsImported    int      `json:"networksImported"`
	ItemsImported  int      `json:"itemsImported"`
	PollsImported  int      `json:"pollingsImported"`
	EventsImported int      `json:"eventsImported"`
	SkippedEntries int      `json:"skippedEntries"`
	Warnings       []string `json:"warnings"`
}

// Options configures import behavior.
type Options struct {
	ImportEventLogs bool
}

// ImportFCDatabase reads a legacy TWSNMP FC bbolt DB and imports its topology,
// configurations, pollings, and logs into the target TWSNMP NEO DataStore.
func ImportFCDatabase(ctx context.Context, fcDBPath string, dest datastore.DataStore, opts Options) (*MigrationReport, error) {
	if dest == nil {
		return nil, datastore.ErrInvalidParams
	}
	if _, err := os.Stat(fcDBPath); err != nil {
		return nil, fmt.Errorf("source db not found: %w", err)
	}

	srcDB, err := bbolt.Open(fcDBPath, 0400, &bbolt.Options{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("open legacy db: %w", err)
	}
	defer srcDB.Close()

	report := &MigrationReport{
		SourceDB: fcDBPath,
		Warnings: make([]string, 0),
	}

	err = srcDB.View(func(tx *bbolt.Tx) error {
		// 1. Import Map Configuration
		if b := tx.Bucket([]byte("config")); b != nil {
			if data := b.Get([]byte("mapConf")); data != nil {
				var mapConf datastore.MapConfEnt
				if err := json.Unmarshal(data, &mapConf); err == nil {
					// Ensure log format is updated to parquet
					if mapConf.LogFormat == "" {
						mapConf.LogFormat = "parquet"
					}
					if err := dest.SaveMapConf(ctx, &mapConf); err != nil {
						report.Warnings = append(report.Warnings, fmt.Sprintf("failed to save map config: %v", err))
					}
				}
			}
		}

		// 2. Import Nodes
		if b := tx.Bucket([]byte("nodes")); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var node datastore.NodeEnt
				if err := json.Unmarshal(v, &node); err != nil {
					report.SkippedEntries++
					report.Warnings = append(report.Warnings, fmt.Sprintf("invalid node JSON for key %s: %v", string(k), err))
					return nil
				}
				if node.ID == "" {
					node.ID = string(k)
				}
				if err := dest.SaveNode(ctx, &node); err != nil {
					report.SkippedEntries++
					report.Warnings = append(report.Warnings, fmt.Sprintf("failed to save node %s: %v", node.ID, err))
				} else {
					report.NodesImported++
				}
				return nil
			})
		}

		// 3. Import Lines
		if b := tx.Bucket([]byte("lines")); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var line datastore.LineEnt
				if err := json.Unmarshal(v, &line); err != nil {
					report.SkippedEntries++
					report.Warnings = append(report.Warnings, fmt.Sprintf("invalid line JSON for key %s: %v", string(k), err))
					return nil
				}
				if line.ID == "" {
					line.ID = string(k)
				}
				if err := dest.SaveLine(ctx, &line); err != nil {
					report.SkippedEntries++
					report.Warnings = append(report.Warnings, fmt.Sprintf("failed to save line %s: %v", line.ID, err))
				} else {
					report.LinesImported++
				}
				return nil
			})
		}

		// 4. Import Networks
		if b := tx.Bucket([]byte("networks")); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var nw datastore.NetworkEnt
				if err := json.Unmarshal(v, &nw); err != nil {
					report.SkippedEntries++
					report.Warnings = append(report.Warnings, fmt.Sprintf("invalid network JSON for key %s: %v", string(k), err))
					return nil
				}
				if nw.ID == "" {
					nw.ID = string(k)
				}
				if err := dest.SaveNetwork(ctx, &nw); err != nil {
					report.SkippedEntries++
					report.Warnings = append(report.Warnings, fmt.Sprintf("failed to save network %s: %v", nw.ID, err))
				} else {
					report.NwsImported++
				}
				return nil
			})
		}

		// 5. Import Draw Items
		if b := tx.Bucket([]byte("items")); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var item datastore.DrawItemEnt
				if err := json.Unmarshal(v, &item); err != nil {
					report.SkippedEntries++
					report.Warnings = append(report.Warnings, fmt.Sprintf("invalid item JSON for key %s: %v", string(k), err))
					return nil
				}
				if item.ID == "" {
					item.ID = string(k)
				}
				if err := dest.SaveDrawItem(ctx, &item); err != nil {
					report.SkippedEntries++
					report.Warnings = append(report.Warnings, fmt.Sprintf("failed to save draw item %s: %v", item.ID, err))
				} else {
					report.ItemsImported++
				}
				return nil
			})
		}

		// 6. Import Pollings
		if b := tx.Bucket([]byte("pollings")); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var p datastore.PollingEnt
				if err := json.Unmarshal(v, &p); err != nil {
					report.SkippedEntries++
					report.Warnings = append(report.Warnings, fmt.Sprintf("invalid polling JSON for key %s: %v", string(k), err))
					return nil
				}
				if p.ID == "" {
					p.ID = string(k)
				}
				if err := dest.SavePolling(ctx, &p); err != nil {
					report.SkippedEntries++
					report.Warnings = append(report.Warnings, fmt.Sprintf("failed to save polling %s: %v", p.ID, err))
				} else {
					report.PollsImported++
				}
				return nil
			})
		}

		// 7. Optional Event Logs
		if opts.ImportEventLogs {
			if b := tx.Bucket([]byte("eventlog")); b != nil {
				_ = b.ForEach(func(k, v []byte) error {
					var ev datastore.EventLogEnt
					if err := json.Unmarshal(v, &ev); err == nil {
						if err := dest.AddEventLog(ctx, &ev); err == nil {
							report.EventsImported++
						}
					}
					return nil
				})
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("read legacy db contents: %w", err)
	}

	slog.Info("Completed TWSNMP FC database import",
		"nodes", report.NodesImported,
		"lines", report.LinesImported,
		"pollings", report.PollsImported,
		"items", report.ItemsImported,
		"warnings", len(report.Warnings),
	)

	return report, nil
}
