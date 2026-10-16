const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const specPath = path.resolve(__dirname, '../../backend/contracts/openapi/openapi.yaml');
const currentSchemaPath = path.resolve(__dirname, '../src/lib/api/schema.d.ts');
const tempSchemaPath = path.resolve(__dirname, '../src/lib/api/schema.tmp.d.ts');

console.log('Validating TypeScript API client matches OpenAPI spec...');

if (!fs.existsSync(specPath)) {
  console.error(`Error: OpenAPI spec not found at ${specPath}`);
  process.exit(1);
}

try {
  execSync(`npx openapi-typescript "${specPath}" -o "${tempSchemaPath}"`, { stdio: 'pipe' });

  if (!fs.existsSync(currentSchemaPath)) {
    console.error('Error: src/lib/api/schema.d.ts does not exist. Run "npm run generate:api"');
    fs.unlinkSync(tempSchemaPath);
    process.exit(1);
  }

  const currentContent = fs.readFileSync(currentSchemaPath, 'utf8');
  const tempContent = fs.readFileSync(tempSchemaPath, 'utf8');

  fs.unlinkSync(tempSchemaPath);

  if (currentContent !== tempContent) {
    console.error('FAILED: src/lib/api/schema.d.ts is out of date with backend/contracts/openapi/openapi.yaml.');
    console.error('Run "npm run generate:api" to synchronize the API client.');
    process.exit(1);
  }

  console.log('SUCCESS: TypeScript API client is up to date with OpenAPI specification.');
  process.exit(0);
} catch (err) {
  console.error('Error checking API client:', err.message);
  if (fs.existsSync(tempSchemaPath)) {
    fs.unlinkSync(tempSchemaPath);
  }
  process.exit(1);
}
