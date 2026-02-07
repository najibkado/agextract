package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors AI tool session directories and fires a callback
// when a file has been quiescent (no writes) for a configured duration.
type Watcher struct {
	fsWatcher  *fsnotify.Watcher
	tracker    *QuiescenceTracker
	sources    []SourcePath
	done       chan struct{}
}

// New creates a Watcher that monitors the given source paths.
// The callback fires when a relevant file stops changing for 60s.
func New(sources []SourcePath, callback func(filePath, tool string)) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating fsnotify watcher: %w", err)
	}

	w := &Watcher{
		fsWatcher: fsw,
		tracker:   NewQuiescenceTracker(DefaultQuiescenceDuration, callback),
		sources:   sources,
		done:      make(chan struct{}),
	}

	// Add watch paths
	for _, sp := range sources {
		if err := w.addRecursive(sp.Path); err != nil {
			fmt.Printf("Warning: could not watch %s (%s): %v\n", sp.Tool, sp.Path, err)
		}
	}

	// Start event loop
	go w.loop()

	return w, nil
}

func (w *Watcher) addRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if info.IsDir() {
			return w.fsWatcher.Add(path)
		}
		return nil
	})
}

func (w *Watcher) loop() {
	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				tool := w.toolForPath(event.Name)
				if tool != "" && w.isRelevantFile(event.Name, tool) {
					w.tracker.Touch(event.Name, tool)
				}

				// Watch newly created directories
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						w.fsWatcher.Add(event.Name)
					}
				}
			}

		case _, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}

		case <-w.done:
			return
		}
	}
}

func (w *Watcher) toolForPath(filePath string) string {
	for _, sp := range w.sources {
		if strings.HasPrefix(filePath, sp.Path) {
			return sp.Tool
		}
	}
	return ""
}

func (w *Watcher) isRelevantFile(filePath string, tool string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	base := strings.ToLower(filepath.Base(filePath))

	switch tool {
	case "claudecode":
		return ext == ".jsonl"
	case "cursor", "windsurf":
		return base == "state.vscdb"
	case "copilot":
		return ext == ".json"
	}
	return false
}

// Close stops the watcher.
func (w *Watcher) Close() {
	w.tracker.Stop()
	close(w.done)
	w.fsWatcher.Close()
}
