/**
 * Generated by orval v7.10.0 🍺
 * Do not edit manually.
 * BrowserGrid API
 * BrowserGrid is a distributed browser automation platform that provides scalable browser sessions and worker pool management.
 * OpenAPI spec version: 1.0
 */
import type { WorkPool } from './workPool';

/**
 * Response containing a list of work pools
 */
export interface WorkPoolListResponse {
  pools?: WorkPool[];
  total?: number;
}
