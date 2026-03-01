package session

import (
	"fmt"
	"time"

	"github.com/Kishanmp3/breaklog/internal/ai"
	"github.com/Kishanmp3/breaklog/internal/db"
)

// Start opens a new active session for the project.
// It records the current shadow repo HEAD as the snapshot hash.
func Start(database *db.DB, project *db.Project) (*db.Session, error) {
	// Get the current HEAD of the shadow repo to use as the session baseline.
	snapshotHash, err := GetHeadHash(project.ShadowRepo, project.Path)
	if err != nil {
		// Not fatal — we'll do a full diff from an empty tree.
		snapshotHash = ""
	}

	sess, err := database.CreateSession(project.ID, snapshotHash)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return sess, nil
}

// Close ends an active session: generates a diff, calls the AI, and stores
// the summary. apiKey may be empty (summary will be skipped with a note).
func Close(database *db.DB, sess *db.Session, project *db.Project, apiKey string) error {
	now := time.Now().UTC()

	diffText, files, err := GenerateDiff(project.Path, project.ShadowRepo, sess.SnapshotHash)
	if err != nil {
		// Non-fatal: store what we have.
		diffText = ""
		files = nil
	}

	var summary string
	if apiKey != "" && diffText != "" {
		summary, err = ai.SummarizeSession(diffText, project.Name, apiKey)
		if err != nil {
			summary = fmt.Sprintf("[summary unavailable: %s]", err)
		}
	} else if diffText == "" {
		summary = "No code changes detected in this session."
	} else {
		summary = "[AI summarization skipped — no API key configured]"
	}

	if err := database.CloseSession(sess.ID, now, sess.StartedAt, diffText, summary, files); err != nil {
		return fmt.Errorf("close session in db: %w", err)
	}
	return nil
}
