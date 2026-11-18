# Servidor dummy de desenvolvimento

`tools/dummy/main.go` é um utilitário **exclusivamente manual** de desenvolvimento,
separado do produto e da suíte de testes. Usa apenas biblioteca padrão; não é uma
dependência de runtime. A tag `//go:build ignore` o exclui da descoberta de pacotes
para build, lint e testes automáticos. O Make executa o arquivo explicitamente,
sem incluir o utilitário no binário httpgo.

## Iniciar e encerrar

Na raiz do repositório:

```sh
make serve
# Ou o alias:
make dummy
# Sem Make, inclusive no Windows:
go run ./tools/dummy/main.go
```

O processo fica em primeiro plano e escuta **somente em `127.0.0.1:8080`**.
Encerre com **Ctrl+C**: o servidor fecha o listener e permite até cinco segundos
para requisições em andamento terminarem; depois força o fechamento.

A porta 8080 deve estar livre. Se já estiver ocupada, o comando falha com um
diagnóstico; não encerra outro processo nem escolhe outra porta silenciosamente.
O servidor não é iniciado automaticamente em `make build`, `make test` ou E2E.

## Endpoints

As respostas usam `Content-Type: application/json`; `/echo` preserva o Content-Type
declarado na requisição, quando houver.

| Método | Caminho | Status | Body |
|---|---|---|---|
| GET | `/health` | 200 | `{"status":"ok"}` |
| GET | `/inexistente` | 404 | `{"error":"not found"}` |
| GET | `/error` | 500 | `{"error":"dummy server error"}` |
| GET | Qualquer outro caminho | 404 | `{"error":"not found"}` |
| POST | `/products` | 201 | `{"id":1}` |
| GET | `/products/1` | 200 | `{"id":1,"name":"Pizza","price":4500}` |
| PUT / PATCH | `/products/1` | 200 | `{"id":1}` |
| DELETE | `/products/1` | 204 | Sem body |
| GET / POST / PUT / PATCH / DELETE | `/echo` | 200 | Bytes exatos do JSON recebido (ou vazio) |
| Método não aceito na rota | — | 405 | `{"error":"method not allowed"}`, header `Allow` com métodos da rota |

`/products`, `/products/1` e `/echo` aceitam body vazio ou um valor JSON válido,
limitado a **1 MiB**. JSON inválido retorna 400, body acima do limite retorna 413;
os erros não reproduzem a entrada. As operações com payload retornam os headers
`X-Dummy-Method` (método recebido) e `X-Dummy-Body-Bytes` (quantidade de bytes).

Os produtos são **fixtures sem persistência**, não um CRUD real: PUT/PATCH apenas
confirmam recebimento, DELETE não remove dados e GET `/products/1` sempre devolve
o mesmo exemplo. Não são atribuídos IDs novos. Os caminhos são exatos; por
exemplo GET `/products/2` continua retornando 404.

Queries não modificam os resultados. O servidor não ecoa query ou headers
sensíveis (Authorization/Proxy-Authorization/Cookie/Set-Cookie), nem mantém log
de requisições. **Somente `/echo` devolve o body ao cliente**, intencionalmente,
para inspeção manual: use apenas dados fictícios, nunca credenciais reais. Não
há TLS, autenticação, configuração externa ou promessa de uso em produção.

## Executar os cenários

Com o servidor aberto em um terminal, use outro:

```sh
make build
bin/httpgo test testdata/scenarios/scenario_get_status.httpgo
bin/httpgo test testdata/scenarios/scenario_unvalidated_status.httpgo
bin/httpgo test testdata/scenarios/scenario_json_inline.httpgo
bin/httpgo test testdata/scenarios/scenario_dummy_methods.httpgo
```

O primeiro cenário valida 200 e 404; o segundo recebe 500, aceita a resposta e
informa **sem validação de status**. O terceiro valida o POST 201 da demonstração;
o quarto envia POST, PUT, PATCH, JSON multilinha para `/echo`, DELETE sem body
e um GET final. Todos terminam com código 0.

A CLI continua mostrando somente resultados/status, não bodies. Para inspecionar
a resposta de `/echo` e seus headers, use, por exemplo:

```sh
curl -i http://localhost:8080/echo \
  -H 'Content-Type: application/json' \
  --data-binary '{"name":"Pizza","price":4500}'
```

O body da resposta terá os mesmos bytes enviados; `X-Dummy-Method: POST` e
`X-Dummy-Body-Bytes: 29` confirmam método e tamanho. O Content-Type declarado
é preservado. Nenhum body ou header sensível é impresso no terminal do servidor.

## Fora da suíte automatizada

`go test ./...`, `go build ./...` e `make lint` não carregam esse utilitário.
O servidor na porta 8080 é iniciado somente por ação manual, nunca por build,
testes, E2E ou CI. Os testes existentes continuam com seus próprios servidores
`httptest`, sem depender de `make serve`.

Há testes **opt-in** para validar o dummy sem ocupar 8080. Execute explicitamente,
a partir da raiz:

```sh
go test -race -cover ./tools/dummy/main.go ./tools/dummy/main_test.go
go vet ./tools/dummy/main.go ./tools/dummy/main_test.go
```

Eles usam HTTP local em portas dinâmicas, cobrem métodos/status, bytes do echo,
headers selecionados, erros/limite de payload e os quatro cenários acima pelo
binário real. O teste compila seu próprio binário temporário; não depende de
`bin/httpgo`. A tag `ignore` também mantém o arquivo de testes fora da descoberta
automática. Não há novo target ou alteração do gate padrão.
