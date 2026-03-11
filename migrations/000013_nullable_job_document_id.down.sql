DELETE FROM processing_jobs WHERE document_id IS NULL;
ALTER TABLE processing_jobs ALTER COLUMN document_id SET NOT NULL;
