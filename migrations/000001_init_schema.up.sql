CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- organizations
CREATE TABLE organizations (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  name TEXT NOT NULL,
  slug TEXT UNIQUE NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- users
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  email TEXT UNIQUE NOT NULL,
  name TEXT,
  role TEXT NOT NULL CHECK (role IN ('admin', 'editor', 'viewer')),
  password_hash TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- git_connections
CREATE TABLE git_connections (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  provider TEXT NOT NULL CHECK (provider IN ('github', 'gitlab')),
  access_token TEXT NOT NULL,
  repo_url TEXT NOT NULL,
  default_branch TEXT NOT NULL DEFAULT 'main',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- clusters
CREATE TABLE clusters (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  git_conn_id UUID REFERENCES git_connections(id) ON DELETE SET NULL,
  name TEXT NOT NULL,
  stage TEXT NOT NULL CHECK (stage IN ('dev', 'staging', 'prod')),
  type TEXT NOT NULL CHECK (type IN ('controlplane', 'worker')),
  dns_name TEXT,
  cluster_size TEXT NOT NULL DEFAULT 'medium' CHECK (cluster_size IN ('small', 'medium', 'large')),
  config_json JSONB NOT NULL DEFAULT '{}',
  agent_token TEXT UNIQUE,
  git_path TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- cluster_services
CREATE TABLE cluster_services (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  service_name TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('enabled', 'disabled')),
  config_json JSONB,
  UNIQUE(cluster_id, service_name)
);

-- cluster_status
CREATE TABLE cluster_status (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  flux_report JSONB,
  last_seen_at TIMESTAMPTZ,
  sync_ready BOOLEAN,
  sync_revision TEXT,
  components_ok INT,
  components_total INT,
  helmreleases_running INT,
  helmreleases_failing INT,
  kustomizations_running INT,
  kustomizations_failing INT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- generation_history
CREATE TABLE generation_history (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  cluster_id UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  triggered_by UUID REFERENCES users(id) ON DELETE SET NULL,
  git_commit_sha TEXT,
  status TEXT NOT NULL CHECK (status IN ('pending', 'committed', 'failed')),
  error_message TEXT,
  manifests_json JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- audit_log
CREATE TABLE audit_log (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  resource_type TEXT,
  resource_id UUID,
  details_json JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_users_org_id ON users(org_id);
CREATE INDEX idx_users_email ON users(email);

CREATE INDEX idx_organizations_slug ON organizations(slug);

CREATE INDEX idx_git_connections_org_id ON git_connections(org_id);

CREATE INDEX idx_clusters_org_id ON clusters(org_id);
CREATE INDEX idx_clusters_agent_token ON clusters(agent_token);

CREATE INDEX idx_cluster_services_cluster_id ON cluster_services(cluster_id);

CREATE INDEX idx_cluster_status_cluster_id ON cluster_status(cluster_id);

CREATE INDEX idx_generation_history_cluster_id ON generation_history(cluster_id);

CREATE INDEX idx_audit_log_org_id ON audit_log(org_id);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at);
