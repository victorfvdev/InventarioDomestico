# Estoque — Slice C

Inventário doméstico com núcleo de produtos/movimentações e **carga inicial por foto (RN01)** com confirmação humana.

## O que este slice entrega

- API Go (monólito modular) + PostgreSQL
- Produtos com status **estimado** vs **confirmado**
- Movimentações imutáveis: entrada manual, ajuste de inventário, carga inicial
- Ajuste de inventário **não** é consumo
- Fluxo A: foto → sugestões de visão → revisão → confirmação → inventário
- Fallback mock local quando `OPENAI_API_KEY` está ausente

## Pré-requisitos

- Go 1.22+
- Node.js 20+
- PostgreSQL 16 (local ou via Docker)

## Subir o banco

Com Docker:

```bash
docker compose up -d postgres
```

Sem Docker (exemplo local já criado):

```bash
# usuário/senha/db: estoque / estoque / estoque
```

## Configuração

```bash
cp .env.example .env
# opcional: preencha OPENAI_API_KEY para visão real
```

## API (porta 3847)

```bash
cd api
export DATABASE_URL=postgres://estoque:estoque@127.0.0.1:5432/estoque?sslmode=disable
export HTTP_ADDR=:3847
export CORS_ORIGINS=http://127.0.0.1:3848,http://localhost:3848
go run ./cmd/server
```

Healthcheck: `GET http://127.0.0.1:3847/health`

## Web (porta 3848)

```bash
cd web
export NEXT_PUBLIC_API_URL=http://127.0.0.1:3847
npm install
npm run dev -- -p 3848 -H 127.0.0.1
```

Abra [http://127.0.0.1:3848](http://127.0.0.1:3848).

## Fluxo rápido

1. Em **Carga por foto**, envie qualquer imagem de despensa
2. Revise/edite as sugestões (mock se não houver chave)
3. Confirme — só então o saldo é criado como **confirmado**
4. Use **Entrada manual** e **Ajustar** na lista; veja o histórico em **Movimentações**

## Fora deste slice

NFC-e MT, baixa por texto, receitas, voz, multi-usuário e lista de compras.
