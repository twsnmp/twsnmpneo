package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
	bolt "go.etcd.io/bbolt"
)

// MigrationReport summarizes the results of importing legacy TWSNMP FC data.
type MigrationReport struct {
	SourceDir        string   `json:"sourceDir"`
	SourceDB         string   `json:"sourceDB"`
	DestDir          string   `json:"destDir"`
	DestDB           string   `json:"destDB"`
	SourceDBSize     int64    `json:"sourceDBSize"`
	DestDBSize       int64    `json:"destDBSize"`
	SizeReductionPct float64  `json:"sizeReductionPct"`

	NodesImported  int `json:"nodesImported"`
	LinesImported  int `json:"linesImported"`
	NwsImported    int `json:"networksImported"`
	ItemsImported  int `json:"itemsImported"`
	PollsImported  int `json:"pollingsImported"`
	EventsImported int `json:"eventsImported"`
	UsersImported  int `json:"usersImported"`

	FilesCopied    []string `json:"filesCopied"`
	SkippedEntries int      `json:"skippedEntries"`
	Warnings       []string `json:"warnings"`
}

// Options configures import behavior.
type Options struct {
	ImportEventLogs bool
	Force           bool
	DestDir         string
}

// CopyFCAssets copies necessary user assets (images/icons, geoip, extmibs, mib.txt, cmd)
// from legacy TWSNMP FC data directory to TWSNMP NEO target data directory.
func CopyFCAssets(srcDir, destDir string) ([]string, error) {
	if srcDir == "" || destDir == "" {
		return nil, nil
	}
	srcInfo, err := os.Stat(srcDir)
	if err != nil || !srcInfo.IsDir() {
		return nil, nil
	}

	copied := make([]string, 0)

	// 1. Copy icons / images -> destDir/images/
	targetImgDir := filepath.Join(destDir, "images")
	for _, iconFolder := range []string{"icons", "images"} {
		srcImgDir := filepath.Join(srcDir, iconFolder)
		if fi, err := os.Stat(srcImgDir); err == nil && fi.IsDir() {
			if err := os.MkdirAll(targetImgDir, 0755); err == nil {
				entries, _ := os.ReadDir(srcImgDir)
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					ext := strings.ToLower(filepath.Ext(entry.Name()))
					switch ext {
					case ".png", ".jpg", ".jpeg", ".svg", ".gif", ".webp", ".ico":
						srcFile := filepath.Join(srcImgDir, entry.Name())
						dstFile := filepath.Join(targetImgDir, entry.Name())
						if err := copySingleFile(srcFile, dstFile, 0644); err == nil {
							copied = append(copied, filepath.Join("images", entry.Name()))
						}
					}
				}
			}
		}
	}

	// 2. Copy geoip.mmdb
	srcGeoIP := filepath.Join(srcDir, "geoip.mmdb")
	if fi, err := os.Stat(srcGeoIP); err == nil && !fi.IsDir() {
		dstGeoIP := filepath.Join(destDir, "geoip.mmdb")
		if err := copySingleFile(srcGeoIP, dstGeoIP, 0644); err == nil {
			copied = append(copied, "geoip.mmdb")
		}
	}

	// 3. Copy extmibs/ directory
	srcExtMibs := filepath.Join(srcDir, "extmibs")
	if fi, err := os.Stat(srcExtMibs); err == nil && fi.IsDir() {
		dstExtMibs := filepath.Join(destDir, "extmibs")
		_ = os.MkdirAll(dstExtMibs, 0755)
		entries, _ := os.ReadDir(srcExtMibs)
		for _, entry := range entries {
			if !entry.IsDir() {
				srcFile := filepath.Join(srcExtMibs, entry.Name())
				dstFile := filepath.Join(dstExtMibs, entry.Name())
				if err := copySingleFile(srcFile, dstFile, 0644); err == nil {
					copied = append(copied, filepath.Join("extmibs", entry.Name()))
				}
			}
		}
	}

	// 4. Copy mib.txt
	srcMibTxt := filepath.Join(srcDir, "mib.txt")
	if fi, err := os.Stat(srcMibTxt); err == nil && !fi.IsDir() {
		dstMibTxt := filepath.Join(destDir, "mib.txt")
		if err := copySingleFile(srcMibTxt, dstMibTxt, 0644); err == nil {
			copied = append(copied, "mib.txt")
		}
	}

	// 5. Copy cmd/ directory (custom scripts, preserving executable permissions)
	srcCmdDir := filepath.Join(srcDir, "cmd")
	if fi, err := os.Stat(srcCmdDir); err == nil && fi.IsDir() {
		dstCmdDir := filepath.Join(destDir, "cmd")
		_ = os.MkdirAll(dstCmdDir, 0755)
		entries, _ := os.ReadDir(srcCmdDir)
		for _, entry := range entries {
			if !entry.IsDir() {
				srcFile := filepath.Join(srcCmdDir, entry.Name())
				dstFile := filepath.Join(dstCmdDir, entry.Name())
				if err := copySingleFile(srcFile, dstFile, 0755); err == nil {
					copied = append(copied, filepath.Join("cmd", entry.Name()))
				}
			}
		}
	}

	return copied, nil
}

func copySingleFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// ImportFCDatabase reads a legacy TWSNMP FC bbolt DB and imports its topology,
// configurations, users, and logs into the target TWSNMP NEO DataStore.
func ImportFCDatabase(ctx context.Context, fcDBPath string, dest datastore.DataStore, opts Options) (*MigrationReport, error) {
	if dest == nil {
		return nil, datastore.ErrInvalidParams
	}
	srcStat, err := os.Stat(fcDBPath)
	if err != nil {
		return nil, fmt.Errorf("source db not found: %w", err)
	}

	srcDB, err := bolt.Open(fcDBPath, 0400, &bolt.Options{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("open legacy db: %w", err)
	}
	defer srcDB.Close()

	report := &MigrationReport{
		SourceDB:     fcDBPath,
		SourceDBSize: srcStat.Size(),
		Warnings:     make([]string, 0),
		FilesCopied:  make([]string, 0),
	}

	destDir := opts.DestDir
	if destDir == "" {
		destDir = "./data"
	}

	err = srcDB.View(func(tx *bolt.Tx) error {
		// 1. Import Configurations from 'config' bucket
		if b := tx.Bucket([]byte("config")); b != nil {
			// mapConf
			if data := b.Get([]byte("mapConf")); data != nil {
				var mapConf datastore.MapConfEnt
				if err := json.Unmarshal(data, &mapConf); err == nil {
					if mapConf.LogFormat == "" {
						mapConf.LogFormat = "parquet"
					}
					if err := dest.SaveMapConf(ctx, &mapConf); err != nil {
						report.Warnings = append(report.Warnings, fmt.Sprintf("failed to save map config: %v", err))
					}
				}

				// Check if BackImage exists inside FC's mapConf JSON structure
				var fcMapConf struct {
					BackImage *struct {
						Path   string `json:"Path"`
						X      int    `json:"X"`
						Y      int    `json:"Y"`
						Width  int    `json:"Width"`
						Height int    `json:"Height"`
						Color  any    `json:"Color"`
					} `json:"BackImage"`
				}
				if err := json.Unmarshal(data, &fcMapConf); err == nil && fcMapConf.BackImage != nil {
					bi := &datastore.BackImageEnt{
						Path:   fcMapConf.BackImage.Path,
						X:      fcMapConf.BackImage.X,
						Y:      fcMapConf.BackImage.Y,
						Width:  fcMapConf.BackImage.Width,
						Height: fcMapConf.BackImage.Height,
					}
					_ = dest.SaveBackImage(ctx, bi)
				}
			}
			// notifyConf
			if data := b.Get([]byte("notifyConf")); data != nil {
				var notifyConf datastore.NotifyConfEnt
				if err := json.Unmarshal(data, &notifyConf); err == nil {
					if err := dest.SaveNotifyConf(ctx, &notifyConf); err != nil {
						report.Warnings = append(report.Warnings, fmt.Sprintf("failed to save notify config: %v", err))
					}
				}
			}
			// locConf
			if data := b.Get([]byte("locConf")); data != nil {
				var locConf datastore.LocConfEnt
				if err := json.Unmarshal(data, &locConf); err == nil {
					if err := dest.SaveLocConf(ctx, &locConf); err != nil {
						report.Warnings = append(report.Warnings, fmt.Sprintf("failed to save loc config: %v", err))
					}
				}
			}
			// backImage (either JSON BackImageEnt or raw binary bytes from FC)
			if data := b.Get([]byte("backImage")); data != nil && len(data) > 0 {
				var backImage datastore.BackImageEnt
				if err := json.Unmarshal(data, &backImage); err == nil && (backImage.Path != "" || backImage.Width > 0) {
					if err := dest.SaveBackImage(ctx, &backImage); err != nil {
						report.Warnings = append(report.Warnings, fmt.Sprintf("failed to save backImage: %v", err))
					}
				} else if len(data) > 8 {
					// Raw image binary from TWSNMP FC
					targetImgDir := filepath.Join(destDir, "images")
					_ = os.MkdirAll(targetImgDir, 0755)
					imgExt := ".png"
					if len(data) > 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
						imgExt = ".jpg"
					}
					fallbackName := "backimage_map" + imgExt
					dstFallback := filepath.Join(targetImgDir, fallbackName)
					_ = os.WriteFile(dstFallback, data, 0644)
					report.FilesCopied = append(report.FilesCopied, filepath.Join("images", fallbackName))

					curBI, _ := dest.GetBackImage(ctx)
					savePath := "/api/map/image/" + fallbackName
					if curBI != nil && curBI.Path != "" {
						customName := filepath.Base(curBI.Path)
						if customName != "" && customName != "." && customName != "/" {
							dstCustom := filepath.Join(targetImgDir, customName)
							_ = os.WriteFile(dstCustom, data, 0644)
							report.FilesCopied = append(report.FilesCopied, filepath.Join("images", customName))
							savePath = "/api/map/image/" + customName
						}
					}

					bi := &datastore.BackImageEnt{
						Path: savePath,
					}
					if curBI != nil {
						bi.X = curBI.X
						bi.Y = curBI.Y
						bi.Width = curBI.Width
						bi.Height = curBI.Height
					}
					_ = dest.SaveBackImage(ctx, bi)
				}
			}
			// discoverConf
			if data := b.Get([]byte("discoverConf")); data != nil {
				var discConf datastore.DiscoverConfEnt
				if err := json.Unmarshal(data, &discConf); err == nil {
					if err := dest.SaveDiscoverConf(ctx, &discConf); err != nil {
						report.Warnings = append(report.Warnings, fmt.Sprintf("failed to save discover config: %v", err))
					}
				}
			}
			// customIcons
			if data := b.Get([]byte("customIcons")); data != nil {
				var icons []*datastore.IconEnt
				if err := json.Unmarshal(data, &icons); err == nil {
					if err := dest.SaveCustomIcons(ctx, icons); err != nil {
						report.Warnings = append(report.Warnings, fmt.Sprintf("failed to save custom icons: %v", err))
					}
				}
			}
		}

		// 1.5 Extract images from 'images' bucket (if stored inside bbolt in FC)
		if b := tx.Bucket([]byte("images")); b != nil {
			targetImgDir := filepath.Join(destDir, "images")
			_ = os.MkdirAll(targetImgDir, 0755)
			_ = b.ForEach(func(k, v []byte) error {
				if len(v) > 0 {
					name := filepath.Base(string(k))
					dstPath := filepath.Join(targetImgDir, name)
					if err := os.WriteFile(dstPath, v, 0644); err == nil {
						report.FilesCopied = append(report.FilesCopied, filepath.Join("images", name))
					}
				}
				return nil
			})
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

		// 7. Import Users
		if b := tx.Bucket([]byte("users")); b != nil {
			_ = b.ForEach(func(k, v []byte) error {
				var u datastore.UserEnt
				if err := json.Unmarshal(v, &u); err == nil && u.User != "" {
					if err := dest.SaveUser(ctx, &u); err == nil {
						report.UsersImported++
					}
				}
				return nil
			})
		}

		// 8. Optional Event Logs
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

	slog.Info("Completed TWSNMP FC database extraction",
		"nodes", report.NodesImported,
		"lines", report.LinesImported,
		"pollings", report.PollsImported,
		"items", report.ItemsImported,
		"users", report.UsersImported,
		"warnings", len(report.Warnings),
	)

	return report, nil
}

// RunFCMigration executes the complete migration from a TWSNMP FC datadir (or DB file)
// to a new TWSNMP NEO data directory.
func RunFCMigration(ctx context.Context, srcPath, destDir string, opts Options) (*MigrationReport, error) {
	if srcPath == "" || destDir == "" {
		return nil, fmt.Errorf("both source path and destination directory are required")
	}

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return nil, fmt.Errorf("source path not found: %w", err)
	}

	var srcDir, srcDBPath string
	if srcInfo.IsDir() {
		srcDir = srcPath
		// Search for twsnmpfc.db or twsnmp.db
		candidates := []string{
			filepath.Join(srcDir, "twsnmpfc.db"),
			filepath.Join(srcDir, "twsnmp.db"),
			filepath.Join(srcDir, "data", "twsnmpfc.db"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				srcDBPath = c
				break
			}
		}
		if srcDBPath == "" {
			return nil, fmt.Errorf("no TWSNMP FC database found in %s (expected twsnmpfc.db)", srcDir)
		}
	} else {
		srcDBPath = srcPath
		srcDir = filepath.Dir(srcPath)
	}

	destDBPath := filepath.Join(destDir, "twsnmpneo.db")

	// Check if target DB already exists
	if _, err := os.Stat(destDBPath); err == nil {
		if !opts.Force {
			return nil, fmt.Errorf("target database already exists at %s (use -force to overwrite)", destDBPath)
		}
		_ = os.Remove(destDBPath)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("create destination directory: %w", err)
	}

	// 1. Copy Asset Files (icons -> images, geoip, extmibs, cmd)
	copiedFiles, err := CopyFCAssets(srcDir, destDir)
	if err != nil {
		slog.Warn("Failed copying some assets", "error", err)
	}

	// 2. Initialize new clean bbolt DataStore
	destStore, err := bbolt.New(destDBPath)
	if err != nil {
		return nil, fmt.Errorf("create new neo datastore: %w", err)
	}
	defer destStore.Close()

	// 3. Extract and import DB contents
	report, err := ImportFCDatabase(ctx, srcDBPath, destStore, opts)
	if err != nil {
		return nil, fmt.Errorf("import fc database: %w", err)
	}

	report.SourceDir = srcDir
	report.DestDir = destDir
	report.DestDB = destDBPath
	report.FilesCopied = copiedFiles

	// 4. Calculate DB compaction stats
	if destStat, err := os.Stat(destDBPath); err == nil {
		report.DestDBSize = destStat.Size()
		if report.SourceDBSize > 0 {
			diff := report.SourceDBSize - report.DestDBSize
			if diff > 0 {
				report.SizeReductionPct = (float64(diff) / float64(report.SourceDBSize)) * 100.0
			}
		}
	}

	return report, nil
}
