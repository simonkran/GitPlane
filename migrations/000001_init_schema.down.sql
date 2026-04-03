DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS generation_history;
DROP TABLE IF EXISTS cluster_status;
DROP TABLE IF EXISTS cluster_services;
DROP TABLE IF EXISTS clusters;
DROP TABLE IF EXISTS git_connections;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;

DROP EXTENSION IF EXISTS "pgcrypto";
DROP EXTENSION IF EXISTS "uuid-ossp";
