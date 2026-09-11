# Feeder Site — League of Legends Stats Platform

Plataforma de estatísticas de League of Legends (perfis, histórico de partidas e tier
lists), inspirada em ferramentas como U.GG, construída para suportar alto volume de
consultas à Riot API sem estourar o rate limit e com processamento pesado de
estatísticas rodando em background.

## Arquitetura

| Camada | Tecnologia | Papel |
| --- | --- | --- |
| Frontend | Next.js (React) | SSR para indexação de perfis/tier lists e UI |
| Backend API | Go (Gin) | Fan-out/fan-in para a Riot API, rate limiting, endpoints |
| Banco de dados | PostgreSQL | Contas, partidas, participantes, materialized views |
| Cache | Redis | Cache de respostas da Riot API e tokens do rate limiter |
| Fila | RabbitMQ | Ingestão assíncrona do histórico de partidas (MATCH-V5) |
| Infra | Docker, Traefik, Cloudflare | Containerização, reverse proxy/SSL, WAF/DDoS |

### Padrões usados

- **Fan-Out / Fan-In**: busca de perfil dispara chamadas concorrentes para
  SUMMONER-V4, LEAGUE-V4 e MATCH-V5, aguarda todas e monta a resposta.
- **Worker pools**: a API responde o perfil instantaneamente e delega a extração
  detalhada de partidas para workers em background, consumindo da fila RabbitMQ.
- **Rate limiter centralizado**: middleware com Redis controla a cota de
  requisições à Riot API (ex.: 100 req/2min na dev key) antes de sofrer bloqueio.
- **Materialized views**: tier lists globais são pré-calculadas via views
  materializadas / jobs noturnos no Postgres, nunca calculadas em tempo real.

## Estrutura do repositório

```
feeder_site/
├── apps/
│   └── web/           # Frontend Next.js
├── services/
│   └── api/           # Backend Go (Gin)
├── infra/              # Configs de Traefik, migrations, etc.
└── docker-compose.yml  # Postgres, Redis, RabbitMQ para desenvolvimento
```

## Roadmap (etapas de desenvolvimento)

- [x] Etapa 1 — Scaffold do monorepo, docker-compose e esqueleto do backend Go
- [x] Etapa 2 — Cliente da Riot API + rate limiter centralizado (Redis)
- [x] Etapa 3 — Endpoint de busca de perfil (fan-out/fan-in: Summoner + League + Match)
- [x] Etapa 4 — Worker pool de ingestão de partidas via RabbitMQ
- [x] Etapa 5 — Modelagem do PostgreSQL (contas, partidas, participantes) + migrations
- [x] Etapa 6 — Materialized views e job noturno de tier list
- [x] Etapa 7 — Frontend Next.js (perfil, histórico, tier list)
- [ ] Etapa 8 — Infra de produção (Traefik, Cloudflare, deploy)

## Desenvolvimento local

```bash
docker compose up -d          # sobe Postgres, Redis e RabbitMQ
cd services/api
go run ./cmd/api               # sobe a API em :8080
```

Frontend (em outro terminal):

```bash
cd apps/web
cp .env.example .env           # API_BASE_URL aponta pra API acima
npm install
npm run dev                    # http://localhost:3000
```
