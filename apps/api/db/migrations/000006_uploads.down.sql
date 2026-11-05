ALTER TABLE submissions
    DROP COLUMN file_preview_url,
    DROP COLUMN file_mime,
    DROP COLUMN file_name;
ALTER TABLE events DROP COLUMN banner_url;
ALTER TABLE profiles DROP COLUMN avatar_url;
DROP TABLE uploads;
