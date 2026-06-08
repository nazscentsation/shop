// cleanup.go — background job for unverified account maintenance.
//
// Runs every 24 hours:
//   - Day 7:  sends a reminder email to unverified accounts
//   - Day 15: deletes unverified accounts and notifies them
package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/nazscentsation/shop/internal/database"
	"github.com/nazscentsation/shop/internal/email"
)

// StartCleanup starts the background goroutine. Call once from main.
func StartCleanup(db *database.DB, mailer *email.Mailer, siteURL string) {
	go func() {
		// Run once shortly after startup, then every 24 h
		time.Sleep(2 * time.Minute)
		for {
			runCleanup(db, mailer, siteURL)
			time.Sleep(24 * time.Hour)
		}
	}()
}

func runCleanup(db *database.DB, mailer *email.Mailer, siteURL string) {
	ctx := context.Background()

	// ── Send 7-day reminder ───────────────────────────────────────
	// Target accounts that registered between 7d0h and 8d0h ago (24-h window → one reminder)
	reminderRows, err := db.QueryContext(ctx,
		`SELECT email, first_name FROM users
		 WHERE email_verified = FALSE
		   AND created_at < NOW() - INTERVAL '7 days'
		   AND created_at > NOW() - INTERVAL '8 days'`)
	if err != nil {
		slog.Error("cleanup: reminder query failed", "err", err)
	} else {
		defer reminderRows.Close()
		for reminderRows.Next() {
			var em, name string
			if err := reminderRows.Scan(&em, &name); err != nil {
				continue
			}
			go mailer.Send(em, "Please verify your Nazscentsation account",
				"Hi "+name+",\n\nYour account has not been verified yet. "+
					"Please verify your email within the next 8 days or your account will be removed.\n\n"+
					"Verify here: "+siteURL+"/verify-email.html\n\n"+
					"If you need a new link, visit: "+siteURL+"/login.html\n\n"+
					"— Nazscentsation")
			slog.Info("cleanup: sent 7-day reminder", "email", em)
		}
	}

	// ── Delete 15-day unverified accounts ────────────────────────
	deleteRows, err := db.QueryContext(ctx,
		`DELETE FROM users
		 WHERE email_verified = FALSE
		   AND created_at < NOW() - INTERVAL '15 days'
		 RETURNING email, first_name`)
	if err != nil {
		slog.Error("cleanup: delete query failed", "err", err)
		return
	}
	defer deleteRows.Close()
	for deleteRows.Next() {
		var em, name string
		if err := deleteRows.Scan(&em, &name); err != nil {
			continue
		}
		go mailer.Send(em, "Your Nazscentsation account has been removed",
			"Hi "+name+",\n\n"+
				"Your account was removed because your email address was not verified within 15 days of registration.\n\n"+
				"You are welcome to register again at: "+siteURL+"\n\n"+
				"— Nazscentsation")
		slog.Info("cleanup: deleted unverified account", "email", em)
	}
}
