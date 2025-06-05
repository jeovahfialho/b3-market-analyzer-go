#!/bin/bash

# Script de setup do banco de dados

set -e

echo "🚀 Configurando banco de dados B3 Market Analyzer..."

# Variáveis de ambiente
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_NAME=${DB_NAME:-b3_market}
DB_USER=${DB_USER:-b3user}
DB_PASS=${DB_PASS:-b3pass}

# Cria banco se não existir
echo "📦 Criando banco de dados..."
PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U postgres -tc "SELECT 1 FROM pg_database WHERE datname = '$DB_NAME'" | grep -q 1 || \
PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U postgres -c "CREATE DATABASE $DB_NAME"

# Cria usuário se não existir
echo "👤 Criando usuário..."
PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U postgres -tc "SELECT 1 FROM pg_user WHERE usename = '$DB_USER'" | grep -q 1 || \
PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U postgres -c "CREATE USER $DB_USER WITH PASSWORD '$DB_PASS'"

# Concede permissões
echo "🔐 Configurando permissões..."
PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE $DB_NAME TO $DB_USER"

# Executa schema
echo "📋 Criando schema..."
PGPASSWORD=$DB_PASS psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME < scripts/schema.sql

echo "✅ Banco de dados configurado com sucesso!"