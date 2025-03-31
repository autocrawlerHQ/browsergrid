import sqlite3 from 'sqlite3';
import { open, Database } from 'sqlite';
import fs from 'fs/promises';
import path from 'path';
import { Session, SafetyCheck, Action } from '../types';

export class SessionStore {
  private db: Database<sqlite3.Database> | null = null;
  private screenshotsDir: string;
  private dbPath: string;

  constructor(dbPath: string, screenshotsDir: string) {
    this.dbPath = dbPath;
    this.screenshotsDir = screenshotsDir;
  }
  
  /**
   * Initialize the database connection and create tables
   */
  async initialize(): Promise<void> {
    // Create screenshots directory if it doesn't exist
    await fs.mkdir(this.screenshotsDir, { recursive: true });
    
    // Open database connection
    this.db = await open({
      filename: this.dbPath,
      driver: sqlite3.Database
    });
    
    // Create sessions table
    await this.db.exec(`
      CREATE TABLE IF NOT EXISTS sessions (
        id TEXT PRIMARY KEY,
        created_at TEXT NOT NULL,
        last_active_at TEXT NOT NULL,
        status TEXT NOT NULL,
        config TEXT NOT NULL,
        environment_type TEXT NOT NULL,
        environment_options TEXT,
        current_state TEXT,
        history TEXT,
        metadata TEXT
      )
    `);
  }
  
  /**
   * Create a new session
   */
  async createSession(session: Session): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');
    
    await this.db.run(
      `INSERT INTO sessions 
       (id, created_at, last_active_at, status, config, environment_type, 
        environment_options, current_state, history, metadata) 
       VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      [
        session.id,
        session.createdAt.toISOString(),
        session.lastActiveAt.toISOString(),
        session.status,
        JSON.stringify(session.agentConfig),
        session.environmentType,
        JSON.stringify(session.environmentOptions || {}),
        JSON.stringify(session.currentState),
        JSON.stringify(session.history),
        JSON.stringify(session.metadata || {})
      ]
    );
  }
  
  /**
   * Get a session by ID
   */
  async getSession(sessionId: string): Promise<Session | null> {
    if (!this.db) throw new Error('Database not initialized');
    
    const row = await this.db.get(
      'SELECT * FROM sessions WHERE id = ?',
      [sessionId]
    );
    
    if (!row) return null;
    
    // Parse JSON fields
    return {
      id: row.id,
      createdAt: new Date(row.created_at),
      lastActiveAt: new Date(row.last_active_at),
      status: row.status,
      agentConfig: JSON.parse(row.config),
      environmentType: row.environment_type,
      environmentOptions: JSON.parse(row.environment_options || '{}'),
      currentState: JSON.parse(row.current_state),
      history: JSON.parse(row.history),
      metadata: JSON.parse(row.metadata || '{}')
    };
  }
  
  /**
   * Update session
   */
  async updateSession(sessionId: string, updates: Partial<Session>): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');
    
    // Get current session to merge updates
    const current = await this.getSession(sessionId);
    if (!current) throw new Error(`Session not found: ${sessionId}`);
    
    // Update last active timestamp
    const lastActiveAt = new Date();
    
    // Merge updates with current session
    const updated = {
      ...current,
      ...updates,
      lastActiveAt
    };
    
    // Update in database
    await this.db.run(
      `UPDATE sessions SET 
        last_active_at = ?,
        status = ?,
        config = ?,
        environment_type = ?,
        environment_options = ?,
        current_state = ?,
        history = ?,
        metadata = ?
       WHERE id = ?`,
      [
        lastActiveAt.toISOString(),
        updated.status,
        JSON.stringify(updated.agentConfig),
        updated.environmentType,
        JSON.stringify(updated.environmentOptions || {}),
        JSON.stringify(updated.currentState),
        JSON.stringify(updated.history),
        JSON.stringify(updated.metadata || {}),
        sessionId
      ]
    );
  }
  
  /**
   * Store screenshot for a session
   */
  async storeScreenshot(sessionId: string, screenshot: Buffer): Promise<string> {
    // Create session-specific directory
    const sessionDir = path.join(this.screenshotsDir, sessionId);
    await fs.mkdir(sessionDir, { recursive: true });
    
    // Generate filename
    const filename = `${Date.now()}.png`;
    const filepath = path.join(sessionDir, filename);
    
    // Write file
    await fs.writeFile(filepath, screenshot);
    
    // Return relative path
    return path.join(sessionId, filename);
  }
  
  /**
   * Get a screenshot
   */
  async getScreenshot(relativePath: string): Promise<Buffer> {
    const filepath = path.join(this.screenshotsDir, relativePath);
    return fs.readFile(filepath);
  }
  
  /**
   * Delete session
   */
  async deleteSession(sessionId: string): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');
    
    // Delete session from database
    await this.db.run('DELETE FROM sessions WHERE id = ?', [sessionId]);
    
    // Delete screenshots directory for this session
    const sessionDir = path.join(this.screenshotsDir, sessionId);
    try {
      await fs.rm(sessionDir, { recursive: true, force: true });
    } catch (error) {
      console.warn(`Failed to delete screenshots for session ${sessionId}:`, error);
    }
  }
  
  /**
   * Clean up expired sessions
   */
  async cleanupExpiredSessions(maxAgeHours: number = 24): Promise<void> {
    if (!this.db) throw new Error('Database not initialized');
    
    const cutoff = new Date();
    cutoff.setHours(cutoff.getHours() - maxAgeHours);
    
    // Get expired sessions
    const expired = await this.db.all(
      'SELECT id FROM sessions WHERE last_active_at < ?',
      [cutoff.toISOString()]
    );
    
    // Delete each expired session
    for (const row of expired) {
      await this.deleteSession(row.id);
    }
  }
  
  /**
   * Close the database connection
   */
  async close(): Promise<void> {
    if (this.db) {
      await this.db.close();
      this.db = null;
    }
  }
  
  /**
   * Generate a unique session ID
   */
  generateSessionId(): string {
    return `sess_${Date.now()}_${Math.random().toString(36).substring(2, 15)}`;
  }
}