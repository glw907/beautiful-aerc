-- ANALYZE statistics from poplar's 100000-message perf corpus.
-- Regenerate by seeding the corpus with POPLAR_PERF_FULL=1 and copying
-- the .stat1.sql file the seeder writes beside the cached master.
ANALYZE;
DELETE FROM sqlite_stat1;
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('account', 'sqlite_autoindex_account_1', '1 1');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('body', NULL, '100000');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('calendar', 'idx_calendar_account', '1 1');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('contact_card', 'idx_contact_card_account_server', '0 0 0');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('event', 'idx_event_calendar', '5000 5000');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('mailbox', 'idx_mailbox_account', '4 4');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('mailbox', 'idx_mailbox_account_server', '0 0 0');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('message', 'idx_message_account_server', '0 0 0');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('message', 'idx_message_thread', '100000 100000 4 1');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('message_fts_config', 'message_fts_config', '1 1');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('message_fts_data', NULL, '87552');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('message_fts_docsize', NULL, '100000');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('message_fts_idx', 'message_fts_idx', '17343 1335 1');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('message_mailbox', 'idx_message_mailbox_list', '100000 25000 1 1');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('message_mailbox', 'idx_message_mailbox_unread', '19616 4904 1 1');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('message_mailbox', 'sqlite_autoindex_message_mailbox_1', '100000 1 1');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('occurrence', 'idx_occurrence_local_date', '50000 64');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('occurrence', 'idx_occurrence_start_utc', '50000 1');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('occurrence', 'sqlite_autoindex_occurrence_1', '50000 10 1');
INSERT INTO sqlite_stat1 (tbl, idx, stat) VALUES ('schema_version', NULL, '1');
ANALYZE sqlite_schema;
