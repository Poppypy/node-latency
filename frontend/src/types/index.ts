export interface NodeDTO {
  index: number
  name: string
  host: string
  port: number
  scheme: string
  region: string
}

export interface TestResult {
  index: number
  done: boolean
  pass: boolean
  err: string
  avgMs: number
  maxMs: number
  latencyMs: number[]
  attempts: number
  successful: number
}

export interface TestProgress {
  total: number
  done: number
  passed: number
  running: boolean
}

export interface RegionRule {
  pattern: string
  region: string
}

export interface TestSettings {
  attempts: number
  threshold: number      // nanoseconds
  timeout: number        // nanoseconds
  concurrency: number
  requireAll: boolean
  stopOnFail: boolean
  dedup: boolean
  rename: boolean
  renameFmt: string
  regionRules: RegionRule[]
  excludeEnabled: boolean
  excludeKeywords: string[]
  latencyName: boolean
  latencyFmt: string
  ipRename: boolean
  ipLookupURL: string
  ipLookupTimeout: number  // nanoseconds
  ipNameFmt: string
  useCoreTest: boolean
  corePath: string
  coreTestURL: string
}
