CREATE TABLE IF NOT EXISTS capability_registry (
  name VARCHAR(50) PRIMARY KEY, description TEXT, avg_seconds INTEGER, is_active BOOLEAN DEFAULT TRUE
);
INSERT INTO capability_registry (name, description, avg_seconds) VALUES
  ('research', 'Web research with sources', 45),
  ('summarize', 'Summarize documents or text', 15),
  ('extract', 'Extract structured data', 20),
  ('write', 'Write content from brief', 30),
  ('translate', 'Translate text', 15),
  ('analyze', 'Analyze with cross-reference', 45)
ON CONFLICT (name) DO NOTHING;
