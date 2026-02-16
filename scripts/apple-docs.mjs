#!/usr/bin/env node

/**
 * apple-docs CLI - Search Apple Developer Documentation, frameworks, APIs, and WWDC videos
 * Wraps @kimsungwhee/apple-docs-mcp as a standalone CLI tool
 */

// Prevent MCP server auto-start and set working directory
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const skillDir = resolve(__dirname, '..');

// Must chdir BEFORE importing package so WWDC data paths resolve correctly
process.chdir(skillDir);
process.env.NODE_ENV = 'test';

import { createRequire } from 'module';

// Ensure node_modules is findable
const require = createRequire(resolve(skillDir, 'package.json'));

// ─── Argument Parsing ───────────────────────────────────────────────

function parseArgs(argv) {
  const positional = [];
  const flags = {};
  let i = 0;
  while (i < argv.length) {
    const arg = argv[i];
    if (arg.startsWith('--')) {
      const key = arg.slice(2);
      const next = argv[i + 1];
      if (next === undefined || next.startsWith('--')) {
        flags[key] = true;
        i++;
      } else {
        flags[key] = next;
        i += 2;
      }
    } else {
      positional.push(arg);
      i++;
    }
  }
  return { positional, flags };
}

function toBool(val, def = true) {
  if (val === undefined) return def;
  if (val === true || val === 'true') return true;
  if (val === false || val === 'false') return false;
  return def;
}

function toInt(val, def) {
  if (val === undefined) return def;
  const n = parseInt(val, 10);
  return isNaN(n) ? def : n;
}

// ─── Load Package ────────────────────────────────────────────────────

let Server, toolHandlers;

async function loadPackage() {
  try {
    const pkg = await import(require.resolve('@kimsungwhee/apple-docs-mcp'));
    Server = pkg.default;

    // Try importing handlers directly
    const handlersPath = require.resolve('@kimsungwhee/apple-docs-mcp')
      .replace(/index\.js$/, 'tools/handlers.js');
    const handlers = await import(handlersPath);
    toolHandlers = handlers.toolHandlers;
  } catch (e) {
    console.error('Package not installed. Run: bash scripts/setup.sh');
    console.error('Detail:', e.message);
    process.exit(1);
  }
}

// ─── Tool Call Helper ────────────────────────────────────────────────

let serverInstance = null;

async function getServer() {
  if (!serverInstance) {
    serverInstance = new Server();
  }
  return serverInstance;
}

async function callTool(toolName, args) {
  const server = await getServer();
  const handler = toolHandlers[toolName];
  if (!handler) {
    console.error(`Unknown tool: ${toolName}`);
    process.exit(1);
  }
  try {
    const result = await handler(args, server);
    if (result.isError) {
      console.error(result.content.map(c => c.text).join('\n'));
      process.exit(1);
    }
    console.log(result.content.map(c => c.text).join('\n'));
  } catch (e) {
    console.error(`Error: ${e.message}`);
    process.exit(1);
  }
}

// ─── Commands ────────────────────────────────────────────────────────

const USAGE = `apple-docs - Apple Developer Documentation CLI

USAGE:
  apple-docs <command> [arguments] [flags]

COMMANDS:
  search <query>              Search Apple docs
  doc <url>                   Get documentation content
  technologies                List Apple technologies
  symbols <framework>         Search framework symbols
  related <url>               Find related APIs
  references <url>            Resolve API references
  compatibility <url>         Check platform compatibility
  similar <url>               Find similar APIs
  updates                     Get documentation updates
  overviews                   Get technology overviews
  samples                     Browse sample code
  wwdc list                   List WWDC videos
  wwdc search <query>         Search WWDC transcripts/code
  wwdc video <year> <id>      Get WWDC video details
  wwdc code                   Browse WWDC code examples
  wwdc topics                 Browse WWDC topics
  wwdc related <year> <id>    Find related WWDC videos
  wwdc years                  List WWDC years

Run 'apple-docs <command> --help' for command-specific help.`;

async function main() {
  const { positional, flags } = parseArgs(process.argv.slice(2));

  if (positional.length === 0 || flags.help) {
    console.log(USAGE);
    process.exit(0);
  }

  await loadPackage();

  const command = positional[0];
  const rest = positional.slice(1);

  switch (command) {

    // ── search ─────────────────────────────────────────────────
    case 'search': {
      const query = rest.join(' ');
      if (!query) { console.error('Usage: apple-docs search <query> [--type all|documentation|sample]'); process.exit(1); }
      await callTool('search_apple_docs', {
        query,
        type: flags.type || 'all',
      });
      break;
    }

    // ── doc ────────────────────────────────────────────────────
    case 'doc': {
      const url = rest[0];
      if (!url) { console.error('Usage: apple-docs doc <url> [--related] [--refs] [--similar] [--platform]'); process.exit(1); }
      await callTool('get_apple_doc_content', {
        url,
        includeRelatedApis: toBool(flags.related, false),
        includeReferences: toBool(flags.refs, false),
        includeSimilarApis: toBool(flags.similar, false),
        includePlatformAnalysis: toBool(flags.platform, false),
      });
      break;
    }

    // ── technologies ───────────────────────────────────────────
    case 'technologies': {
      await callTool('list_technologies', {
        category: flags.category,
        language: flags.language,
        includeBeta: toBool(flags.beta, true),
        limit: toInt(flags.limit, 200),
      });
      break;
    }

    // ── symbols ────────────────────────────────────────────────
    case 'symbols': {
      const framework = rest[0];
      if (!framework) { console.error('Usage: apple-docs symbols <framework> [--type <type>] [--pattern <pat>] [--language swift|occ] [--limit N]'); process.exit(1); }
      await callTool('search_framework_symbols', {
        framework,
        symbolType: flags.type || 'all',
        namePattern: flags.pattern,
        language: flags.language || 'swift',
        limit: toInt(flags.limit, 50),
      });
      break;
    }

    // ── related ────────────────────────────────────────────────
    case 'related': {
      const url = rest[0];
      if (!url) { console.error('Usage: apple-docs related <url> [--no-inherited] [--no-conformance] [--no-see-also]'); process.exit(1); }
      await callTool('get_related_apis', {
        apiUrl: url,
        includeInherited: !flags['no-inherited'],
        includeConformance: !flags['no-conformance'],
        includeSeeAlso: !flags['no-see-also'],
      });
      break;
    }

    // ── references ─────────────────────────────────────────────
    case 'references': {
      const url = rest[0];
      if (!url) { console.error('Usage: apple-docs references <url> [--max N] [--filter <type>]'); process.exit(1); }
      await callTool('resolve_references_batch', {
        sourceUrl: url,
        maxReferences: toInt(flags.max, 20),
        filterByType: flags.filter || 'all',
      });
      break;
    }

    // ── compatibility ──────────────────────────────────────────
    case 'compatibility': {
      const url = rest[0];
      if (!url) { console.error('Usage: apple-docs compatibility <url> [--mode single|framework] [--related]'); process.exit(1); }
      await callTool('get_platform_compatibility', {
        apiUrl: url,
        compareMode: flags.mode || 'single',
        includeRelated: toBool(flags.related, false),
      });
      break;
    }

    // ── similar ────────────────────────────────────────────────
    case 'similar': {
      const url = rest[0];
      if (!url) { console.error('Usage: apple-docs similar <url> [--depth shallow|medium|deep] [--category <cat>] [--no-alternatives]'); process.exit(1); }
      await callTool('find_similar_apis', {
        apiUrl: url,
        searchDepth: flags.depth || 'medium',
        filterByCategory: flags.category,
        includeAlternatives: !flags['no-alternatives'],
      });
      break;
    }

    // ── updates ────────────────────────────────────────────────
    case 'updates': {
      await callTool('get_documentation_updates', {
        category: flags.category || 'all',
        technology: flags.tech,
        year: flags.year,
        searchQuery: flags.query,
        includeBeta: toBool(flags.beta, true),
        limit: toInt(flags.limit, 50),
      });
      break;
    }

    // ── overviews ──────────────────────────────────────────────
    case 'overviews': {
      await callTool('get_technology_overviews', {
        category: flags.category,
        platform: flags.platform || 'all',
        searchQuery: flags.query,
        includeSubcategories: toBool(flags.subcategories, true),
        limit: toInt(flags.limit, 50),
      });
      break;
    }

    // ── samples ────────────────────────────────────────────────
    case 'samples': {
      await callTool('get_sample_code', {
        framework: flags.framework,
        beta: flags.beta || 'include',
        searchQuery: flags.query || rest.join(' ') || undefined,
        limit: toInt(flags.limit, 50),
      });
      break;
    }

    // ── wwdc ───────────────────────────────────────────────────
    case 'wwdc': {
      const sub = rest[0];
      const subRest = rest.slice(1);

      switch (sub) {
        case 'list':
          await callTool('list_wwdc_videos', {
            year: flags.year,
            topic: flags.topic,
            hasCode: flags['has-code'] !== undefined ? toBool(flags['has-code']) : undefined,
            limit: toInt(flags.limit, 50),
          });
          break;

        case 'search': {
          const q = subRest.join(' ');
          if (!q) { console.error('Usage: apple-docs wwdc search <query> [--in transcript|code|both] [--year Y] [--language L] [--limit N]'); process.exit(1); }
          await callTool('search_wwdc_content', {
            query: q,
            searchIn: flags.in || 'both',
            year: flags.year,
            language: flags.language,
            limit: toInt(flags.limit, 20),
          });
          break;
        }

        case 'video': {
          const year = subRest[0];
          const videoId = subRest[1];
          if (!year || !videoId) { console.error('Usage: apple-docs wwdc video <year> <videoId> [--no-transcript] [--no-code]'); process.exit(1); }
          await callTool('get_wwdc_video', {
            year,
            videoId,
            includeTranscript: !flags['no-transcript'],
            includeCode: !flags['no-code'],
          });
          break;
        }

        case 'code':
          await callTool('get_wwdc_code_examples', {
            framework: flags.framework,
            topic: flags.topic,
            year: flags.year,
            language: flags.language,
            limit: toInt(flags.limit, 30),
          });
          break;

        case 'topics':
          await callTool('browse_wwdc_topics', {
            topicId: flags.topic,
            includeVideos: toBool(flags.videos, true),
            year: flags.year,
            limit: toInt(flags.limit, 20),
          });
          break;

        case 'related': {
          const year = subRest[0];
          const videoId = subRest[1];
          if (!year || !videoId) { console.error('Usage: apple-docs wwdc related <year> <videoId> [--no-explicit] [--no-topic] [--year-related] [--limit N]'); process.exit(1); }
          await callTool('find_related_wwdc_videos', {
            videoId,
            year,
            includeExplicitRelated: !flags['no-explicit'],
            includeTopicRelated: !flags['no-topic'],
            includeYearRelated: toBool(flags['year-related'], false),
            limit: toInt(flags.limit, 15),
          });
          break;
        }

        case 'years':
          await callTool('list_wwdc_years', {});
          break;

        default:
          console.error(`Unknown wwdc subcommand: ${sub}`);
          console.error('Available: list, search, video, code, topics, related, years');
          process.exit(1);
      }
      break;
    }

    default:
      console.error(`Unknown command: ${command}`);
      console.log(USAGE);
      process.exit(1);
  }
}

main().catch(e => {
  console.error(`Fatal: ${e.message}`);
  process.exit(1);
});
