# Configuração YAML — extensão HTTPGo (US-11)

Corpus separado do formato `.hurl` e do diferencial Hurl. Não fornecer estes
arquivos ou flags `--config`/`--render-files` ao Hurl.

Com o dummy manual em execução:

```sh
bin/httpgo test testdata/scenarios/scenario_config_products.hurl --config testdata/config/project.yaml
# Destino alternativo não sigiloso; product.price é preservado:
bin/httpgo test testdata/scenarios/scenario_config_products.hurl --config testdata/config/project.yaml --var api.baseUrl=http://localhost:9090
```

Para descoberta automática, copie `project.yaml` para `.httpgo.yaml` no diretório
corrente. Não há busca em ancestrais. Valores destes exemplos não são credenciais.
Os YAMLs `invalid_*`/`unsupported_*` são negativos: caminhos repetidos,
referência parcial/aninhada ou inválida, âncora/alias e tag explícita retornam 2
antes de qualquer HTTP, inclusive com override CLI.

## Referências de processo — extensão HTTPGo (US-13)

Com `OWNER_TOKEN` fornecido externamente e servidor `/profile` recebendo o header:

```sh
bin/httpgo test testdata/scenarios/scenario_process_profile.hurl --config testdata/config/process_base.yaml --env process
```

`environments/process.yaml` fornece `auth.token: ${OWNER_TOKEN}`. Também aceita
referências na base (`version: 1` obrigatória), sem `--env`. Somente valor string
completo, nome `[A-Za-z_][A-Za-z0-9_]*`; sem composição parcial/listas. Aplica
base < ambiente < CLI antes de consultar referências efetivas. Ausência, mesmo
não usada, é erro 2 antes de HTTP; vazio presente é string vazia. `123`/`true`/`null`
permanecem strings e conteúdo não sofre nova expansão. Não importa variáveis
sem referência. Destino ganha marca secreta antes dos overrides, acumulada com
`secrets`/`--secret`. `--var auth.token=ficticio` pode substituir sem exigir o
processo e sem perder proteção; **nunca forneça credenciais reais em argumentos**.
Protege valores conhecidos, substitutos e derivados, não segredos desconhecidos
ou transformações arbitrárias. Sem dotenv, stdin, `HTTPGO_*` ou JUnit.

Os testes E2E fornecem valores fictícios no ambiente do processo filho, não em
argumentos/fixtures. Os exemplos são separados do corpus diferencial Hurl.
