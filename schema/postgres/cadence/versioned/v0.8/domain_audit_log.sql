ALTER TABLE domain_audit_log ALTER COLUMN state_before DROP NOT NULL;
ALTER TABLE domain_audit_log ALTER COLUMN state_after DROP NOT NULL;
