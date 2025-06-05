# B3 Market Analyzer - Guia de Instalação com Docker 🚀

Sistema de alta performance para análise de dados do mercado B3, implementado em Go com foco em eficiência e escalabilidade.

## 📋 Pré-requisitos

Antes de começar, certifique-se de ter instalado:

- **Docker** 24.x ou superior
- **Docker Compose** 2.x ou superior
- **Git** (para clonar o repositório)

### Verificando as instalações

```bash
# Verificar Docker
docker --version
# Esperado: Docker version 24.x.x ou superior

# Verificar Docker Compose
docker-compose --version
# Esperado: Docker Compose version 2.x.x ou superior
```

## 🚀 Quick Start

### 1. Clone o repositório

```bash
git clone https://github.com/jeovahfialho/b3-market-analyzer-go.git
cd b3-market-analyzer-go
```

### 2. Prepare o ambiente

```bash
# Criar diretório para dados
mkdir -p data

# Criar arquivo de configuração
cp .env.example .env 2>/dev/null || touch .env

# Dar permissões corretas aos scripts
chmod 755 scripts/*.sh 2>/dev/null || true
```

### 3. Configure o arquivo .env

Crie ou edite o arquivo `.env` na raiz do projeto:

```env
# Database
DATABASE_URL=postgres://b3user:b3pass@localhost:5432/b3_market?sslmode=disable
DATABASE_MAX_CONNS=25
DATABASE_MIN_CONNS=5
DATABASE_MAX_CONN_LIFE=1h

# Redis
REDIS_URL=redis://localhost:6379
CACHE_TTL=1h

# Processing
BATCH_SIZE=10000
WORKERS=4

# API
API_HOST=0.0.0.0
API_PORT=8000
LOG_LEVEL=info

# Environment
ENVIRONMENT=development
```

### 4. Subir os containers

```bash
# Entrar no diretório docker
cd docker

# Subir todos os serviços em modo detached
docker-compose up -d

# Ou se quiser ver os logs em tempo real
docker-compose up
```

## 📊 Verificando a instalação

### 1. Verificar status dos containers

```bash
# Ver status dos containers
docker-compose ps
```

Você deve ver algo como:
```
NAME                 STATUS              PORTS
b3_api              running             0.0.0.0:8000->8000/tcp
b3_postgres         running (healthy)   0.0.0.0:5432->5432/tcp
b3_redis            running (healthy)   0.0.0.0:6379->6379/tcp
b3_prometheus       running             0.0.0.0:9090->9090/tcp
b3_grafana          running             0.0.0.0:3000->3000/tcp
```

### 2. Verificar logs

```bash
# Ver logs de todos os serviços
docker-compose logs

# Ver logs específicos
docker-compose logs api
docker-compose logs postgres

# Seguir logs em tempo real
docker-compose logs -f api
```

### 3. Testar a aplicação

```bash
# Testar health check
curl http://localhost:8000/health

# Resposta esperada:
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2024-01-15T10:30:00Z"
}

# Testar readiness
curl http://localhost:8000/ready

# Testar uma query (No início não vai mostrar nada)
curl http://localhost:8000/api/v1/ticker/PETR4
```

## 🌐 Acessando as interfaces

Após subir os containers, você pode acessar:

- **API**: http://localhost:8000
- **Swagger/Docs**: http://localhost:8000/swagger
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 
  - Login: `admin`
  - Senha: `admin`

## 📥 Carregando dados

### Opção 1: Usando a CLI dentro do container

```bash
# Entrar no container da API
docker-compose exec api sh

# Dentro do container, baixar dados dos últimos 7 dias
./b3-analyzer-cli download --days 7

# Carregar os arquivos CSV
./b3-analyzer-cli load data/*.txt

# Sair do container
exit
```

### Opção 2: Executar comandos diretamente

```bash
# Download de dados
docker-compose exec api ./b3-analyzer-cli download --days 7

# Carregar dados
docker-compose exec api ./b3-analyzer-cli load data/*.txt

# Verificar dados carregados
docker-compose exec api ./b3-analyzer-cli query PETR4
```

## 🔍 Testando com dados reais

Após carregar os dados:

```bash
# Consultar agregações de um ticker
curl http://localhost:8000/api/v1/ticker/PETR4

# Com filtro de data
curl "http://localhost:8000/api/v1/ticker/VALE3?start_date=2024-01-01"

# Top volume
curl http://localhost:8000/api/v1/analysis/top-volume

# Estatísticas
curl http://localhost:8000/api/v1/ticker/ITUB4/stats
```

## 🛠️ Comandos úteis

### Gerenciamento dos containers

```bash
# Parar todos os serviços
docker-compose down

# Parar e remover volumes (limpa todos os dados)
docker-compose down -v

# Reconstruir imagens
docker-compose build

# Reconstruir e subir
docker-compose up -d --build

# Reiniciar um serviço específico
docker-compose restart api

# Ver uso de recursos
docker stats
```

### Executar comandos nos containers

```bash
# Acessar shell do container
docker-compose exec api sh

# Executar query no PostgreSQL
docker-compose exec postgres psql -U b3user -d b3_market

# Acessar Redis CLI
docker-compose exec redis redis-cli

# Ver logs do PostgreSQL
docker-compose exec postgres tail -f /var/log/postgresql/postgresql.log
```

### Backup e restore

```bash
# Backup do banco de dados
docker-compose exec postgres pg_dump -U b3user b3_market > backup.sql

# Restore do banco de dados
docker-compose exec -T postgres psql -U b3user b3_market < backup.sql
```

## 🚨 Troubleshooting

### Problema: Portas em uso

```bash
# Verificar se as portas estão livres
lsof -i :8000  # API
lsof -i :5432  # PostgreSQL
lsof -i :6379  # Redis
lsof -i :3000  # Grafana
lsof -i :9090  # Prometheus

# Matar processo usando a porta (exemplo)
kill -9 $(lsof -t -i:8000)
```

### Problema: Container não inicia

```bash
# Ver logs detalhados
docker-compose logs api | grep ERROR

# Verificar saúde dos containers
docker inspect b3_api | grep -i health

# Reconstruir do zero
docker-compose down -v
docker-compose build --no-cache
docker-compose up -d
```

### Problema: Erro de permissão

```bash
# Ajustar permissões
sudo chown -R $USER:$USER .
chmod -R 755 data/
```

### Problema: Falta de memória

```bash
# Verificar memória disponível
docker system df

# Limpar recursos não utilizados
docker system prune -a
```

## 📝 Script helper

Para facilitar o uso diário, crie um arquivo `docker-helper.sh`:

```bash
#!/bin/bash

case "$1" in
  start)
    echo "🚀 Iniciando B3 Market Analyzer..."
    cd docker && docker-compose up -d
    echo "⏳ Aguardando serviços..."
    sleep 10
    echo "✅ Serviços iniciados!"
    echo "📊 API: http://localhost:8000"
    echo "📈 Grafana: http://localhost:3000"
    ;;
  
  stop)
    echo "🛑 Parando serviços..."
    cd docker && docker-compose down
    ;;
  
  logs)
    cd docker && docker-compose logs -f api
    ;;
  
  load-data)
    echo "📥 Carregando dados de exemplo..."
    cd docker
    docker-compose exec api ./b3-analyzer-cli download --days 7
    docker-compose exec api ./b3-analyzer-cli load data/*.csv
    ;;
  
  test)
    echo "🧪 Testando API..."
    curl -s http://localhost:8000/health | jq
    ;;
  
  status)
    cd docker && docker-compose ps
    ;;
  
  clean)
    echo "🧹 Limpando tudo..."
    cd docker && docker-compose down -v
    ;;
  
  rebuild)
    echo "🔨 Reconstruindo..."
    cd docker
    docker-compose down
    docker-compose build --no-cache
    docker-compose up -d
    ;;
  
  *)
    echo "Uso: $0 {start|stop|logs|load-data|test|status|clean|rebuild}"
    exit 1
    ;;
esac
```

Torne o script executável:
```bash
chmod +x docker-helper.sh
```

Uso:
```bash
./docker-helper.sh start      # Iniciar serviços
./docker-helper.sh load-data  # Carregar dados
./docker-helper.sh test       # Testar API
./docker-helper.sh logs       # Ver logs
./docker-helper.sh stop       # Parar serviços
```

## ✅ Verificação final

Se tudo estiver funcionando corretamente, você verá:

1. ✅ Todos os containers rodando (`docker-compose ps`)
2. ✅ Health check retornando "healthy" 
3. ✅ Grafana acessível em http://localhost:3000
4. ✅ Logs sem erros (`docker-compose logs`)

## 🎉 Pronto!

Seu B3 Market Analyzer está rodando com Docker! 🚀🐳

### Próximos passos

1. **Carregar dados reais**: Use a CLI para baixar e carregar dados da B3
2. **Explorar a API**: Acesse http://localhost:8000/swagger
3. **Configurar dashboards**: Acesse o Grafana e crie visualizações
4. **Otimizar performance**: Ajuste as configurações no `.env`

## 📚 Documentação adicional

- [Arquitetura do Sistema](./docs/ARCHITECTURE.md)
- [API Reference](./docs/API.md)
- [Guia de Desenvolvimento](./docs/DEVELOPMENT.md)
- [Performance Tuning](./docs/PERFORMANCE.md)

## 🤝 Suporte

Se encontrar problemas:

1. Verifique a seção de Troubleshooting
2. Consulte os logs: `docker-compose logs`
3. Abra uma issue no GitHub
4. Consulte a documentação do Docker
