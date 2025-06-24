data "external_schema" "gorm" {
  program = [
    "sh", "-c", "cd browsergrid && go run -mod=mod ./cmd/schema-loader",
  ]
}

env "gorm" {
  src = data.external_schema.gorm.url
  dev = "docker://postgres/15/dev"
  migration {
    dir = "file://browsergrid/migrations"
  }
  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
} 