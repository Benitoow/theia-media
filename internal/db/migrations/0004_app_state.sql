-- Small key/value store for things the application remembers about itself, as
-- opposed to things the user configured. The first entry is whether the
-- onboarding screen has been dismissed; the auto-updater will want one for the
-- last version check.
--
-- Kept out of config.json deliberately: that file is the user's to edit, and
-- nothing in it should change by itself.

CREATE TABLE app_state (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
