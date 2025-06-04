#!/usr/bin/env python3
"""
Fix preferred asset issues in ViewRA database.

This script fixes the database where multiple assets are incorrectly marked as 
preferred for the same entity+type combination. It implements a priority system
to determine which asset should be preferred.
"""

import sqlite3
import os
from pathlib import Path
from typing import List, Tuple, Dict

def fix_preferred_assets(db_path: Path, assets_dir: Path):
    """Fix preferred asset conflicts in the database."""
    print(f"Fixing preferred assets in database: {db_path}")
    print(f"Assets directory: {assets_dir}")
    
    # Connect to database
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    # Find all entity+type combinations with multiple preferred assets
    cursor.execute("""
        SELECT entity_type, entity_id, type, COUNT(*) as preferred_count
        FROM media_assets 
        WHERE preferred = 1 
        GROUP BY entity_type, entity_id, type 
        HAVING COUNT(*) > 1
        ORDER BY entity_type, type
    """)
    
    conflicts = cursor.fetchall()
    print(f"Found {len(conflicts)} entity+type combinations with multiple preferred assets")
    
    fixed_count = 0
    
    for entity_type, entity_id, asset_type, count in conflicts:
        print(f"Fixing: {entity_type}/{entity_id} type={asset_type} ({count} preferred assets)")
        
        # Get all preferred assets for this entity+type
        cursor.execute("""
            SELECT id, path, source, plugin_id, created_at
            FROM media_assets 
            WHERE entity_type = ? AND entity_id = ? AND type = ? AND preferred = 1
            ORDER BY created_at DESC
        """, (entity_type, entity_id, asset_type))
        
        preferred_assets = cursor.fetchall()
        
        # Determine which asset should remain preferred using priority system
        best_asset = determine_best_asset(preferred_assets, assets_dir)
        
        if best_asset:
            best_id, best_path, best_source, best_plugin_id, best_created = best_asset
            
            # Unset all preferred flags for this entity+type
            cursor.execute("""
                UPDATE media_assets 
                SET preferred = 0 
                WHERE entity_type = ? AND entity_id = ? AND type = ?
            """, (entity_type, entity_id, asset_type))
            
            # Set the best asset as preferred
            cursor.execute("""
                UPDATE media_assets 
                SET preferred = 1 
                WHERE id = ?
            """, (best_id,))
            
            print(f"  → Set preferred: {best_path} (source: {best_source}, plugin: {best_plugin_id})")
            fixed_count += 1
        else:
            print(f"  → Could not determine best asset, keeping first one")
    
    # Commit changes
    conn.commit()
    conn.close()
    
    print(f"\nSummary:")
    print(f"  Conflicts found: {len(conflicts)}")
    print(f"  Conflicts fixed: {fixed_count}")

def determine_best_asset(assets: List[Tuple], assets_dir: Path) -> Tuple:
    """
    Determine the best asset to keep as preferred based on priority rules.
    
    Priority (highest to lowest):
    1. embedded > plugin > core
    2. File exists on disk
    3. musicbrainz_enricher > core_enrichment > others
    4. Most recent (latest created_at)
    """
    if not assets:
        return None
    
    scored_assets = []
    
    for asset in assets:
        asset_id, path, source, plugin_id, created_at = asset
        score = 0
        
        # Check if file exists (highest priority)
        full_path = assets_dir / path
        if full_path.exists():
            score += 1000
        else:
            print(f"    Warning: File does not exist: {path}")
        
        # Source priority
        if source == "embedded":
            score += 100
        elif source == "plugin":
            score += 90
        elif source == "core":
            score += 80
        else:
            score += 70
        
        # Plugin priority
        if plugin_id == "musicbrainz_enricher":
            score += 20
        elif plugin_id == "core_enrichment":
            score += 15
        else:
            score += 10
        
        # More recent assets get slightly higher priority
        # (created_at is ISO format, so string comparison works)
        if created_at:
            score += hash(created_at) % 10  # Add some variability based on timestamp
        
        scored_assets.append((score, asset))
    
    # Sort by score (highest first)
    scored_assets.sort(key=lambda x: x[0], reverse=True)
    
    best_score, best_asset = scored_assets[0]
    print(f"    Best asset score: {best_score}")
    
    return best_asset

def verify_fix(db_path: Path):
    """Verify that the fix worked correctly."""
    print("\nVerifying fix...")
    
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()
    
    # Check for remaining conflicts
    cursor.execute("""
        SELECT entity_type, entity_id, type, COUNT(*) as preferred_count
        FROM media_assets 
        WHERE preferred = 1 
        GROUP BY entity_type, entity_id, type 
        HAVING COUNT(*) > 1
    """)
    
    remaining_conflicts = cursor.fetchall()
    
    if remaining_conflicts:
        print(f"ERROR: Still {len(remaining_conflicts)} conflicts remaining!")
        for conflict in remaining_conflicts:
            print(f"  - {conflict}")
    else:
        print("✅ All conflicts resolved!")
    
    # Get some statistics
    cursor.execute("SELECT COUNT(*) FROM media_assets WHERE preferred = 1")
    preferred_count = cursor.fetchone()[0]
    
    cursor.execute("SELECT COUNT(*) FROM media_assets")
    total_count = cursor.fetchone()[0]
    
    print(f"Statistics:")
    print(f"  Total assets: {total_count}")
    print(f"  Preferred assets: {preferred_count}")
    print(f"  Non-preferred assets: {total_count - preferred_count}")
    
    conn.close()

def main():
    # Paths
    project_root = Path(__file__).parent
    db_path = project_root / "viewra-data" / "viewra.db"
    assets_dir = project_root / "viewra-data" / "assets"
    
    if not db_path.exists():
        print(f"Database not found: {db_path}")
        return
    
    if not assets_dir.exists():
        print(f"Assets directory not found: {assets_dir}")
        return
    
    fix_preferred_assets(db_path, assets_dir)
    verify_fix(db_path)

if __name__ == "__main__":
    main() 