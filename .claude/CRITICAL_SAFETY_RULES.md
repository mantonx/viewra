# CRITICAL SAFETY RULES

## NEVER DELETE USER DATA

**ABSOLUTELY FORBIDDEN:**
- NEVER run `rm -rf` on user directories containing media, documents, or any personal files
- NEVER delete music libraries, video libraries, photo collections, or any user content
- NEVER assume it's okay to delete user files "for testing"

**When testing is needed:**
- Use temporary test directories with synthetic/dummy data
- Create small test datasets specifically for testing
- Ask the user explicitly before touching ANY real data
- If in doubt, DO NOT DELETE ANYTHING

**This applies to:**
- Music files and libraries
- Video files and libraries  
- Photo collections
- Documents
- Any user-created content
- Database files containing user data

## If you need to test something that requires fresh data:

1. Create a NEW temporary directory (e.g., `/tmp/test-music`)
2. Copy a SMALL subset of files there for testing
3. Work only in that temporary directory
4. NEVER touch the original files

**Remember:** User data is irreplaceable. There is NO excuse for deleting it.
