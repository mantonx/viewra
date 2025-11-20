#!/bin/bash
# Script to normalize existing sort_title values in the database

# Define the NormalizeSortTitle function equivalent in SQL
# This will strip accents and articles from existing titles

cat <<EOF | sqlite3 data/viewra.db
-- Update movies sort_title by normalizing from title
UPDATE movies
SET sort_title = LOWER(
  TRIM(
    REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
      -- Remove leading articles after stripping accents manually in SQL
      CASE
        WHEN media.title LIKE 'The %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'A %' THEN SUBSTR(media.title, 3)
        WHEN media.title LIKE 'An %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'Le %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'La %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'Les %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'El %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'Los %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'Der %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'Die %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'Das %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'Il %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'Lo %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'De %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'Het %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'O %' THEN SUBSTR(media.title, 3)
        WHEN media.title LIKE 'Os %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'As %' THEN SUBSTR(media.title, 4)
        -- Handle accented article "À"
        WHEN media.title LIKE 'À %' OR media.title LIKE 'à %' THEN SUBSTR(media.title, 3)
        ELSE media.title
      END,
      'à','a'),'á','a'),'â','a'),'ã','a'),'ä','a'),'å','a'),
      'è','e'),'é','e'),'ê','e'),'ë','e'),
      'ì','i'),'í','i'),'î','i'),'ï','i'),
      'ò','o'),'ó','o'),'ô','o'),'õ','o'),'ö','o'),
      'ù','u'),'ú','u'),'û','u'),'ü','u')
  )
)
FROM media
WHERE movies.media_id = media.id;

SELECT 'Updated ' || changes() || ' movie sort_title values';

-- Update TV shows sort_title
UPDATE tv_shows
SET sort_title = LOWER(
  TRIM(
    REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
      CASE
        WHEN title LIKE 'The %' THEN SUBSTR(title, 5)
        WHEN title LIKE 'A %' THEN SUBSTR(title, 3)
        WHEN title LIKE 'An %' THEN SUBSTR(title, 4)
        WHEN title LIKE 'Le %' THEN SUBSTR(title, 4)
        WHEN title LIKE 'La %' THEN SUBSTR(title, 4)
        WHEN title LIKE 'Les %' THEN SUBSTR(title, 5)
        WHEN title LIKE 'El %' THEN SUBSTR(title, 4)
        WHEN title LIKE 'Los %' THEN SUBSTR(title, 5)
        WHEN title LIKE 'Der %' THEN SUBSTR(title, 5)
        WHEN title LIKE 'Die %' THEN SUBSTR(title, 5)
        WHEN title LIKE 'Das %' THEN SUBSTR(title, 5)
        WHEN title LIKE 'Il %' THEN SUBSTR(title, 4)
        WHEN title LIKE 'Lo %' THEN SUBSTR(title, 4)
        WHEN title LIKE 'De %' THEN SUBSTR(title, 4)
        WHEN title LIKE 'Het %' THEN SUBSTR(title, 5)
        WHEN title LIKE 'O %' THEN SUBSTR(title, 3)
        WHEN title LIKE 'Os %' THEN SUBSTR(title, 4)
        WHEN title LIKE 'As %' THEN SUBSTR(title, 4)
        WHEN title LIKE 'À %' OR title LIKE 'à %' THEN SUBSTR(title, 3)
        ELSE title
      END,
      'à','a'),'á','a'),'â','a'),'ã','a'),'ä','a'),'å','a'),
      'è','e'),'é','e'),'ê','e'),'ë','e'),
      'ì','i'),'í','i'),'î','i'),'ï','i'),
      'ò','o'),'ó','o'),'ô','o'),'õ','o'),'ö','o'),
      'ù','u'),'ú','u'),'û','u'),'ü','u')
  )
);

SELECT 'Updated ' || changes() || ' TV show sort_title values';

-- Update music tracks sort_title
UPDATE music_tracks
SET sort_title = LOWER(
  TRIM(
    REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
      CASE
        WHEN media.title LIKE 'The %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'A %' THEN SUBSTR(media.title, 3)
        WHEN media.title LIKE 'An %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'Le %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'La %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'Les %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'El %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'Los %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'Der %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'Die %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'Das %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'Il %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'Lo %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'De %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'Het %' THEN SUBSTR(media.title, 5)
        WHEN media.title LIKE 'O %' THEN SUBSTR(media.title, 3)
        WHEN media.title LIKE 'Os %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'As %' THEN SUBSTR(media.title, 4)
        WHEN media.title LIKE 'À %' OR media.title LIKE 'à %' THEN SUBSTR(media.title, 3)
        ELSE media.title
      END,
      'à','a'),'á','a'),'â','a'),'ã','a'),'ä','a'),'å','a'),
      'è','e'),'é','e'),'ê','e'),'ë','e'),
      'ì','i'),'í','i'),'î','i'),'ï','i'),
      'ò','o'),'ó','o'),'ô','o'),'õ','o'),'ö','o'),
      'ù','u'),'ú','u'),'û','u'),'ü','u')
  )
)
FROM media
WHERE music_tracks.media_id = media.id;

SELECT 'Updated ' || changes() || ' music track sort_title values';
EOF

echo "Sort title normalization complete!"
