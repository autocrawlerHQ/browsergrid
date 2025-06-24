module.exports = {
  browsergrid: {
    input: {
      target: '../browsergrid/docs/swagger.json',
    },
    output: {
      mode: 'tags-split',
      target: './src/lib/api/browserGridAPI.ts',
      schemas: './src/lib/api/model',
      client: 'react-query',
      mock: false,
      override: {
        query: {
          useQuery: true,
          useInfinite: false,
          options: {
            staleTime: 10000,
            refetchOnWindowFocus: false,
          },
        },
        mutator: {
          path: './src/lib/api/mutator.ts',
          name: 'customInstance',
        },
      },
    },
  },
}; 