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
Os demais YAMLs são negativos: caminhos repetidos, referência de processo,
âncora/alias e tag explícita devem retornar 2 antes de qualquer HTTP.
