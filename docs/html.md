# Compartilhar resultados em HTML — US-35 / issue #72

## Exemplo

```bash
httpgo test tests/ --fail-fast --output html --output-file target/report.html
```

Com `01-health.httpgo` aprovado, `02-products.httpgo` recebendo 503 em vez de 200 e
`03-profile.httpgo` não iniciado pelo fail-fast, a CLI retorna **1**, deixa stdout
vazio e informa em stderr, após gravar e fechar:

```text
relatório HTML gerado em "target/report-20261117T193045123Z.html"
```

O horário acima é ilustrativo, não fixo. Abra o caminho **efetivamente informado**
no navegador, mesmo sem internet. O documento é único/autossuficiente: CSS estático
embutido, sem JavaScript, fontes, imagens, links ou estilos externos. O tema é
somente claro, inspirado no Markdown do GitHub, com texto escuro sobre fundo
branco, títulos, tabelas discretas e diagnósticos monoespaçados. Estados sempre
possuem rótulos textuais, não apenas cores.

A demonstração versionada está em [testdata/examples/html_suite/](../testdata/examples/html_suite/):

```bash
httpgo test testdata/examples/html_suite/ --var base_url=http://localhost:8080 --fail-fast --output html --output-file target/report.html
```

Requer um servidor com `/health` e `/profile` retornando 200 e `/products`
retornando 503 para demonstrar a divergência. Sem fail-fast, `/profile` executa.
O exemplo não fornece esses endpoints nem presume que qualquer API local os tenha.

Arquivo único também é suportado:

```bash
httpgo test testdata/examples/html_suite/01-health.httpgo --var base_url=http://localhost:8080 --output html --output-file target/report.html
```

## Destino e nomeação

- `--output html` exige `--output-file <caminho>` não vazio. Ambas aceitam espaço
  ou `=` e podem aparecer em qualquer ordem após `test`, uma vez cada. Ausência
  de destino, repetição, formato inválido ou destino sem formato retorna **2**
  antes de qualquer HTTP. Texto continua padrão; não há `--output text/json`.
- O destino é um **nome-base**, não o nome final: captura uma vez o horário UTC
  na gravação e insere `YYYYMMDDTHHMMSSmmmZ` antes da última extensão. Preserva
  diretório, pontos no nome e extensão; sem extensão, acrescenta ao nome inteiro.
  Ex.: `target/api.results.html` → `target/api.results-20261117T193045123Z.html`.
- A primeira colisão vira `...123Z-1.html`, depois `-2`, usando o primeiro número
  livre (não necessariamente o maior). A criação é exclusiva, inclusive sob
  colisão concorrente, sem truncar candidatos existentes ou o nome-base.
  Erros diferentes de existência não são tratados como colisões.
- Cria pais quando possível com `0700` e arquivos novos com `0600`, sujeitos às
  restrições do sistema/umask. O destino deve ser um nome de arquivo, não diretório.
  A política de substituição do JUnit **não muda**.
- A confirmação usa stderr protegido, somente após escrita **e fechamento**
  bem-sucedidos. Falha da própria confirmação retorna **2**, mesmo com HTML válido.
  Não se insiste em um stream que já falhou.

## Conteúdo e contagens

O relatório recebe os resultados existentes, sem replay HTTP ou reavaliação:

- Resumo agregado, resultado por arquivo, duração e detalhes das requisições
  alcançadas, na ordem da execução. Método, URL protegida, resultado textual,
  tentativas e diagnóstico localizado quando disponível. Divergência identifica
  requisição/assertion, arquivo, linha/coluna e esperado/recebido quando existentes.
- Sucesso, falha de assertion, erro e não executado são distintos. Requisições
  seguintes interrompidas aparecem nas contagens **conhecidas** de não iniciadas,
  sem detalhes ou durações inventados. Arquivo rejeitado no parsing tem erro,
  zero envios e contagem desconhecida; arquivo não analisado por fail-fast ou
  interrupção global é não executado, com total/não iniciadas desconhecidos.
- Agregados cobrem somente requisições conhecidas e mostram quantos arquivos têm
  contagens desconhecidas. Itens interrompidos nunca contam como aprovados.
  `status validados` conta comparações de status aprovadas, não sucesso de todas
  as assertions. Ausência de assertion não é validação; `HTTP *` permanece sem
  comparação numérica.
- Duração lógica da entrada inclui preparação, todas as tentativas, avaliação e
  esperas de polling até o resultado terminal; exclui a apresentação. Há um único
  detalhe terminal, não uma linha por tentativa. Duração não negativa, sem mudar
  timeouts, retry, isolamento, ordem ou fail-fast. Arquivo e suíte preservam suas
  durações existentes; escrita do HTML ocorre depois da execução.

## Sigilo e falhas

Não há bodies nem headers integrais, nem em comentários ou atributos ocultos.
URLs removem credenciais, query inteira e fragmento; nomes, URLs e diagnósticos
são sempre **texto escapado**, inclusive aspas, Unicode e `<script>`. Nenhum dado
variável entra em scripts, CSS ou atributos executáveis.

A proteção conhecida/marcada/derivada é aplicada antes do escaping e reaplicada
após a suíte, inclusive em metadados/diagnósticos de arquivos anteriores a uma
nova descoberta de segredo. stderr incremental só protege o que já era conhecido
no momento da emissão: não é possível redigi-lo retroativamente. Segredos
arbitrários não marcados ou transformações desconhecidas não são detectados.
Não coloque segredos no path; use referências de ambiente e marcações explícitas.
Nunca forneça credenciais por flags, inclusive `--var`. Stdin secreto não é suportado.
YAML hierárquico e interpolação externa continuam **extensões HTTPGo**, não Hurl.

Códigos preservados: **0** sucesso, **1** falha de assertion, **2** demais erros,
com **2 prevalecendo sobre 1**. Havendo resultados, falha de cenário, cancelamento
durante a suíte ou falha de apresentação não impedem tentar gerar HTML dos dados
disponíveis, sem novos envios. Configuração/composição/descoberta inválida ou
cancelamento anterior à execução não abrem relatório nem criam resultados fictícios.
Falha de criação/escrita curta/escrita/fechamento retorna **2**, com diagnóstico
seguro e sem confirmação falsa. A gravação não é transacional e pode deixar
arquivo parcial; não há promessa de remoção ou rollback de efeitos da API.

Fora de escopo: tema escuro, filtros interativos, histórico comparativo,
dashboard, publicação externa, HTML e JUnit simultâneos, exposição integral de
requests/responses ou compatibilidade com flags HTML do Hurl. O corpus Hurl não
é alterado por este formato próprio de apresentação.
