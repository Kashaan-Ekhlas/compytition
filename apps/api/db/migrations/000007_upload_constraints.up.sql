-- Enforce the upload invariants the services already assume, and switch
-- presentation attachments to download-only by clearing legacy preview keys.
UPDATE uploads SET preview_key = '' WHERE mime_type <> 'application/pdf';
UPDATE submissions SET file_preview_url = '' WHERE file_mime <> 'application/pdf';

ALTER TABLE uploads
    ADD CONSTRAINT uploads_event_scope
    CHECK ((kind = 'attachment') = (event_id IS NOT NULL));

ALTER TABLE uploads
    ADD CONSTRAINT uploads_preview_only_pdf
    CHECK (preview_key = '' OR mime_type = 'application/pdf');
