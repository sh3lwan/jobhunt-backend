-- Remembered navigation per ATS host: how the application form is opened (an
-- "Apply" link vs an inline button) and which button is the final submit.
-- Postings on the same ATS (Ashby, Greenhouse, Lever, …) share a layout, so this
-- lets the applier navigate deterministically instead of re-probing button/link
-- candidates on every new form.
CREATE TABLE IF NOT EXISTS form_navigation (
    host       TEXT PRIMARY KEY,
    nav        JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
