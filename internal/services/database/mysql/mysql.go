package mysql

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/models"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/backup/backupmodels"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/services/database"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/tag"
	"github.com/MK-Mods-OFC/Los-Templarios/internal/util/vote"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/multierror"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/permissions"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/stringutil"
	"github.com/MK-Mods-OFC/Los-Templarios/pkg/twitchnotify"

	"github.com/bwmarrin/snowflake"
	_ "github.com/go-sql-driver/mysql"
)

// MysqlMiddleware implements the Database interface for
// MariaDB or MysqlMiddleware.
type MysqlMiddleware struct {
	Db *sql.DB
}

var _ database.Database = (*MysqlMiddleware)(nil)

var guildTables = []string{
	"antiraidJoinlog",
	"antiraidSettings",
	"backups",
	"chanlock",
	"guildapi",
	"guildlog",
	"guilds",
	"karma",
	"karmaBlocklist",
	"karmaRules",
	"karmaSettings",
	"permissions",
	"reports",
	"starboardConfig",
	"starboardEntries",
	"tags",
	"twitchnotify",
	"unbanRequests",
	"verificationQueue",
	"voicelogBlocklist",
	"birthdays",
}

type tableColumn struct {
	Table  string
	Column string
}

var userTables = []tableColumn{
	{"antiraidJoinlog", "userID"},
	{"apitokens", "userID"},
	{"refreshTokens", "userID"},
	{"starboardEntries", "authorID"},
	{"tags", "creatorID"},
	{"unbanRequests", "userID"},
	{"unbanRequests", "processedBy"},
	{"users", "userID"},
	{"birthdays", "userID"},
}

func (m *MysqlMiddleware) setup() (err error) {
	if err = m.Db.Ping(); err != nil {
		return
	}

	tx, err := m.Db.Begin()
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `migrations` (" +
		"`version` int(16) NOT NULL DEFAULT '0'," +
		"`applied` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP()," +
		"`releaseTag` text NOT NULL DEFAULT ''," +
		"`releaseCommit` text NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`version`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `guilds` (" +
		"`guildID` varchar(25) NOT NULL," +
		"`prefix` text NOT NULL DEFAULT ''," +
		"`autorole` text NOT NULL DEFAULT ''," +
		"`autovc` text NOT NULL DEFAULT ''," +
		"`modlogchanID` text NOT NULL DEFAULT ''," +
		"`voicelogchanID` text NOT NULL DEFAULT ''," +
		"`notifyRoleID` text NOT NULL DEFAULT ''," +
		"`ghostPingMsg` text NOT NULL DEFAULT ''," +
		"`jdoodleToken` text NOT NULL DEFAULT ''," +
		"`codeExecEnabled` text NOT NULL DEFAULT ''," +
		"`backup` text NOT NULL DEFAULT ''," +
		"`inviteBlock` text NOT NULL DEFAULT ''," +
		"`joinMsg` text NOT NULL DEFAULT ''," +
		"`leaveMsg` text NOT NULL DEFAULT ''," +
		"`colorReaction` text NOT NULL DEFAULT ''," +
		"`guildlogDisable` text NOT NULL DEFAULT ''," +
		"`requireUserVerification` text NOT NULL DEFAULT ''," +
		"`birthdaychanID` text NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`guildID`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `users` (" +
		"`userID` varchar(25) NOT NULL," +
		"`enableOTA` text NOT NULL DEFAULT '0'," +
		"`verified` text NOT NULL DEFAULT '0'," +
		"`starboardOptout` text NOT NULL DEFAULT '0'," +
		"PRIMARY KEY (`userID`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `permissions` (" +
		"`roleID` varchar(25) NOT NULL," +
		"`guildID` text NOT NULL DEFAULT ''," +
		"`permission` text NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`roleID`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `reports` (" +
		"`id` varchar(25) NOT NULL," +
		"`type` int(11) NOT NULL DEFAULT '0'," +
		"`guildID` text NOT NULL DEFAULT ''," +
		"`executorID` text NOT NULL DEFAULT ''," +
		"`victimID` text NOT NULL DEFAULT ''," +
		"`msg` text NOT NULL DEFAULT ''," +
		"`attachment` text NOT NULL DEFAULT ''," +
		"`timeout` timestamp NULL DEFAULT NULL," +
		"PRIMARY KEY (`id`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `settings` (" +
		"`iid` int(11) NOT NULL AUTO_INCREMENT," +
		"`setting` text NOT NULL DEFAULT ''," +
		"`value` text NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`iid`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `votes` (" +
		"`id` varchar(25) NOT NULL," +
		"`data` mediumtext NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`id`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `twitchnotify` (" +
		"`iid` int(11) NOT NULL AUTO_INCREMENT," +
		"`guildID` text NOT NULL DEFAULT ''," +
		"`channelID` text NOT NULL DEFAULT ''," +
		"`twitchUserID` text NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`iid`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `backups` (" +
		"`iid` int(11) NOT NULL AUTO_INCREMENT," +
		"`guildID` text NOT NULL DEFAULT ''," +
		"`timestamp` bigint(20) NOT NULL DEFAULT CURRENT_TIMESTAMP()," +
		"`fileID` text NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`iid`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `tags` (" +
		"`id` varchar(25) NOT NULL," +
		"`ident` text NOT NULL DEFAULT ''," +
		"`creatorID` text NOT NULL DEFAULT ''," +
		"`guildID` text NOT NULL DEFAULT ''," +
		"`content` text NOT NULL DEFAULT ''," +
		"`created` bigint(20) NOT NULL DEFAULT CURRENT_TIMESTAMP()," +
		"`lastEdit` bigint(20) NOT NULL DEFAULT CURRENT_TIMESTAMP()," +
		"PRIMARY KEY (`id`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `apitokens` (" +
		"`userID` varchar(25) NOT NULL," +
		"`salt` text NOT NULL," +
		"`created` timestamp NOT NULL," +
		"`expires` timestamp NOT NULL," +
		"`lastAccess` timestamp NOT NULL," +
		"`hits` bigint(20) NOT NULL," +
		"PRIMARY KEY (`userID`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `karma` (" +
		"`iid` int(11) NOT NULL AUTO_INCREMENT," +
		"`guildID` text NOT NULL DEFAULT ''," +
		"`userID` text NOT NULL DEFAULT ''," +
		"`value` bigint(20) NOT NULL DEFAULT '0'," +
		"PRIMARY KEY (`iid`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `karmaSettings` (" +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"`state` int(1) NOT NULL DEFAULT '1'," +
		"`emotesInc` text NOT NULL DEFAULT ''," +
		"`emotesDec` text NOT NULL DEFAULT ''," +
		"`tokens` bigint(20) NOT NULL DEFAULT '1'," +
		"`penalty` int(1) NOT NULL DEFAULT '0'," +
		"PRIMARY KEY (`guildID`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `karmaBlocklist` (" +
		"`iid` int(11) NOT NULL AUTO_INCREMENT," +
		"`userID` varchar(25) NOT NULL DEFAULT ''," +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`iid`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `karmaRules` (" +
		"`id` varchar(25) NOT NULL," +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"`trigger` int(8) NOT NULL DEFAULT '0'," +
		"`value` int(32) NOT NULL DEFAULT '0'," +
		"`action` varchar(30) NOT NULL DEFAULT ''," +
		"`argument` text NOT NULL DEFAULT ''," +
		"`checksum` text NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`id`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `chanlock` (" +
		"`chanID` varchar(25) NOT NULL," +
		"`guildID` text NOT NULL DEFAULT ''," +
		"`executorID` text NOT NULL DEFAULT ''," +
		"`permissions` text NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`chanID`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `antiraidSettings` (" +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"`state` int(1) NOT NULL DEFAULT '1'," +
		"`limit` bigint(20) NOT NULL DEFAULT '0'," +
		"`burst` bigint(20) NOT NULL DEFAULT '0'," +
		"`verification` int(1) NOT NULL DEFAULT 0," +
		"PRIMARY KEY (`guildID`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `antiraidJoinlog` (" +
		"`iid` int(11) NOT NULL AUTO_INCREMENT," +
		"`userID` varchar(25) NOT NULL DEFAULT ''," +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"`tag` text NOT NULL DEFAULT ''," +
		"`accountCreated` timestamp NOT NULL DEFAULT 0," +
		"`timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP()," +
		"PRIMARY KEY (`iid`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `unbanRequests` (" +
		"`id` varchar(25) NOT NULL DEFAULT ''," +
		"`userID` varchar(25) NOT NULL DEFAULT ''," +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"`userTag` text NOT NULL DEFAULT ''," +
		"`message` text NOT NULL DEFAULT ''," +
		"`processedBy` varchar(25) NOT NULL DEFAULT ''," +
		"`status` int(8) NOT NULL DEFAULT '0'," +
		"`processed` timestamp," +
		"`processedMessage` text NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`id`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `voicelogBlocklist` (" +
		"`iid` int(11) NOT NULL AUTO_INCREMENT," +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"`channelID` varchar(25) NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`iid`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `starboardConfig` (" +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"`channelID` varchar(25) NOT NULL DEFAULT ''," +
		"`threshold` int(16) NOT NULL DEFAULT '0'," +
		"`emojiID` text NOT NULL DEFAULT ''," +
		"`karmaGain` int(16) NOT NULL DEFAULT '3'," +
		"PRIMARY KEY (`guildID`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `starboardEntries` (" +
		"`messageID` varchar(25) NOT NULL DEFAULT ''," +
		"`starboardID` varchar(25) NOT NULL DEFAULT ''," +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"`channelID` varchar(25) NOT NULL DEFAULT ''," +
		"`authorID` varchar(25) NOT NULL DEFAULT ''," +
		"`content` text NOT NULL DEFAULT ''," +
		"`mediaURLs` text NOT NULL DEFAULT ''," +
		"`score` int(24) NOT NULL DEFAULT '0'," +
		"`deleted` int(1) NOT NULL DEFAULT '0'," +
		"PRIMARY KEY (`messageID`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `refreshTokens` (" +
		"`userID` varchar(25) NOT NULL DEFAULT ''," +
		"`token` text NOT NULL DEFAULT ''," +
		"`expires` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP()," +
		"PRIMARY KEY (`userID`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `guildlog` (" +
		"`id` varchar(25) NOT NULL DEFAULT ''," +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"`module` varchar(30) NOT NULL DEFAULT ''," +
		"`message` text NOT NULL DEFAULT ''," +
		"`severity` int(8) NOT NULL DEFAULT '0'," +
		"`timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP()," +
		"PRIMARY KEY (`id`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `guildapi` (" +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"`enabled` int(1) NOT NULL DEFAULT '0'," +
		"`origins` text NOT NULL DEFAULT ''," +
		"`tokenHash` text NOT NULL DEFAULT ''," +
		"PRIMARY KEY (`guildID`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `verificationQueue` (" +
		"`iid` int(11) NOT NULL AUTO_INCREMENT," +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"`userID` varchar(25) NOT NULL DEFAULT ''," +
		"`timestamp` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP()," +
		"PRIMARY KEY (`iid`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS `birthdays` (" +
		"`iid` int(11) NOT NULL AUTO_INCREMENT," +
		"`guildID` varchar(25) NOT NULL DEFAULT ''," +
		"`userID` varchar(25) NOT NULL DEFAULT ''," +
		"`date` timestamp," +
		"`showYear` int(1) NOT NULL DEFAULT '0'," +
		"PRIMARY KEY (`iid`)" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;")
	if err != nil {
		return
	}

	err = tx.Commit()
	return
}

func (m *MysqlMiddleware) Connect(credentials ...interface{}) (err error) {
	creds := credentials[0].(models.DatabaseCreds)
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?collation=utf8mb4_unicode_ci&parseTime=true",
		creds.User, creds.Password, creds.Host, creds.Database)

	if m.Db, err = sql.Open("mysql", dsn); err != nil {
		return
	}

	err = m.setup()
	return
}

func (m *MysqlMiddleware) Close() {
	if m.Db != nil {
		m.Db.Close()
	}
}

func (m *MysqlMiddleware) getGuildSetting(guildID, key string) (string, error) {
	var value string
	err := m.Db.QueryRow(
		fmt.Sprintf("SELECT %s FROM guilds WHERE guildID = ?", key),
		guildID).Scan(&value)
	err = wrapNotFoundError(err)
	return value, err
}

func (m *MysqlMiddleware) setGuildSetting(guildID, key string, value string) (err error) {
	res, err := m.Db.Exec(
		fmt.Sprintf("UPDATE guilds SET %s = ? WHERE guildID = ?", key),
		value, guildID)
	if err != nil {
		return
	}

	ar, err := res.RowsAffected()
	if err != nil {
		return
	}
	if ar == 0 {
		_, err = m.Db.Exec(
			fmt.Sprintf("INSERT INTO guilds (guildID, %s) VALUES (?, ?)", key),
			guildID, value)
	}

	return nil
}

func (m *MysqlMiddleware) getUserSetting(userID, key string) (string, error) {
	var value string
	err := m.Db.QueryRow(
		fmt.Sprintf("SELECT %s FROM users WHERE userID = ?", key),
		userID).Scan(&value)
	err = wrapNotFoundError(err)
	return value, err
}

func (m *MysqlMiddleware) setUserSetting(userID, key string, value string) (err error) {
	res, err := m.Db.Exec(
		fmt.Sprintf("UPDATE users SET %s = ? WHERE userID = ?", key),
		value, userID)
	if err != nil {
		return
	}

	ar, err := res.RowsAffected()
	if err != nil {
		return
	}
	if ar == 0 {
		_, err = m.Db.Exec(
			fmt.Sprintf("INSERT INTO users (userID, %s) VALUES (?, ?)", key),
			userID, value)
	}

	return nil
}

func (m *MysqlMiddleware) GetGuildPrefix(guildID string) (string, error) {
	val, err := m.getGuildSetting(guildID, "prefix")
	return val, err
}

func (m *MysqlMiddleware) SetGuildPrefix(guildID, newPrefix string) error {
	return m.setGuildSetting(guildID, "prefix", newPrefix)
}

func (m *MysqlMiddleware) GetGuildAutoRole(guildID string) ([]string, error) {
	val, err := m.getGuildSetting(guildID, "autorole")
	if val == "" {
		return []string{}, err
	}
	return strings.Split(val, ";"), err
}

func (m *MysqlMiddleware) SetGuildAutoRole(guildID string, autoRoleIDs []string) error {
	return m.setGuildSetting(guildID, "autorole", strings.Join(autoRoleIDs, ";"))
}

func (m *MysqlMiddleware) GetGuildAutoVC(guildID string) ([]string, error) {
	val, err := m.getGuildSetting(guildID, "autovc")
	if val == "" {
		return []string{}, err
	}
	return strings.Split(val, ";"), err
}

func (m *MysqlMiddleware) SetGuildAutoVC(guildID string, autoVCIDs []string) error {
	return m.setGuildSetting(guildID, "autovc", strings.Join(autoVCIDs, ";"))
}

func (m *MysqlMiddleware) GetGuildModLog(guildID string) (string, error) {
	val, err := m.getGuildSetting(guildID, "modlogchanID")
	return val, err
}

func (m *MysqlMiddleware) SetGuildModLog(guildID, chanID string) error {
	return m.setGuildSetting(guildID, "modlogchanID", chanID)
}

func (m *MysqlMiddleware) GetGuildVoiceLog(guildID string) (string, error) {
	val, err := m.getGuildSetting(guildID, "voicelogchanID")
	return val, err
}

func (m *MysqlMiddleware) SetGuildVoiceLog(guildID, chanID string) error {
	return m.setGuildSetting(guildID, "voicelogchanID", chanID)
}

func (m *MysqlMiddleware) GetGuildNotifyRole(guildID string) (string, error) {
	val, err := m.getGuildSetting(guildID, "notifyRoleID")
	return val, err
}

func (m *MysqlMiddleware) SetGuildNotifyRole(guildID, roleID string) error {
	return m.setGuildSetting(guildID, "notifyRoleID", roleID)
}

func (m *MysqlMiddleware) GetGuildGhostpingMsg(guildID string) (string, error) {
	val, err := m.getGuildSetting(guildID, "ghostPingMsg")
	return val, err
}

func (m *MysqlMiddleware) SetGuildGhostpingMsg(guildID, msg string) error {
	return m.setGuildSetting(guildID, "ghostPingMsg", msg)
}

func (m *MysqlMiddleware) GetGuildColorReaction(guildID string) (enabled bool, err error) {
	val, err := m.getGuildSetting(guildID, "colorReaction")
	return val == "1", err
}

func (m *MysqlMiddleware) SetGuildColorReaction(guildID string, enabled bool) error {
	var val string
	if enabled {
		val = "1"
	}
	return m.setGuildSetting(guildID, "colorReaction", val)
}

func (m *MysqlMiddleware) GetGuildPermissions(guildID string) (map[string]permissions.PermissionArray, error) {
	results := make(map[string]permissions.PermissionArray)
	rows, err := m.Db.Query("SELECT roleID, permission FROM permissions WHERE guildID = ?",
		guildID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var roleID string
		var permission string
		err := rows.Scan(&roleID, &permission)
		if err != nil {
			return nil, err
		}
		results[roleID] = strings.Split(permission, ",")
	}
	return results, nil
}

func (m *MysqlMiddleware) SetGuildRolePermission(guildID, roleID string, p permissions.PermissionArray) error {
	if len(p) == 0 {
		_, err := m.Db.Exec("DELETE FROM permissions WHERE roleID = ?", roleID)
		return err
	}

	pStr := strings.Join(p, ",")
	res, err := m.Db.Exec("UPDATE permissions SET permission = ? WHERE roleID = ? AND guildID = ?",
		pStr, roleID, guildID)
	if err != nil {
		return err
	}
	ar, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ar == 0 {
		_, err = m.Db.Exec("INSERT INTO permissions (roleID, guildID, permission) VALUES (?, ?, ?)",
			roleID, guildID, pStr)
	}
	return err
}

func (m *MysqlMiddleware) GetGuildJdoodleKey(guildID string) (string, error) {
	val, err := m.getGuildSetting(guildID, "jdoodleToken")
	return val, err
}

func (m *MysqlMiddleware) SetGuildJdoodleKey(guildID, key string) error {
	return m.setGuildSetting(guildID, "jdoodleToken", key)
}

func (m *MysqlMiddleware) GetGuildCodeExecEnabled(guildID string) (bool, error) {
	val, err := m.getGuildSetting(guildID, "codeExecEnabled")
	return val == "1", err
}

func (m *MysqlMiddleware) SetGuildCodeExecEnabled(guildID string, enabled bool) error {
	var val string
	if enabled {
		val = "1"
	}
	return m.setGuildSetting(guildID, "codeExecEnabled", val)
}

func (m *MysqlMiddleware) GetGuildBackup(guildID string) (bool, error) {
	val, err := m.getGuildSetting(guildID, "backup")
	return val == "1", err
}

func (m *MysqlMiddleware) SetGuildBackup(guildID string, enabled bool) error {
	var val string
	if enabled {
		val = "1"
	}
	return m.setGuildSetting(guildID, "backup", val)
}

func (m *MysqlMiddleware) GetSetting(setting string) (string, error) {
	var value string
	err := m.Db.QueryRow("SELECT value FROM settings WHERE setting = ?", setting).Scan(&value)
	err = wrapNotFoundError(err)
	return value, err
}

func (m *MysqlMiddleware) SetSetting(setting, value string) error {
	res, err := m.Db.Exec("UPDATE settings SET value = ? WHERE setting = ?", value, setting)
	ar, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ar == 0 {
		_, err = m.Db.Exec("INSERT INTO settings (setting, value) VALUES (?, ?)", setting, value)
	}
	return err
}

func (m *MysqlMiddleware) AddReport(rep *models.Report) error {
	_, err := m.Db.Exec(`
		INSERT INTO reports (id, type, guildID, executorID, victimID, msg, attachment, timeout)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rep.ID, rep.Type, rep.GuildID, rep.ExecutorID, rep.VictimID, rep.Msg, rep.AttachmentURL, rep.Timeout)
	return err
}

func (m *MysqlMiddleware) DeleteReport(id snowflake.ID) error {
	_, err := m.Db.Exec("DELETE FROM reports WHERE id = ?", id)
	return err
}

func (m *MysqlMiddleware) GetReport(id snowflake.ID) (*models.Report, error) {
	rep := new(models.Report)

	row := m.Db.QueryRow(`
		SELECT id, type, guildID, executorID, victimID, msg, attachment, timeout
		FROM reports WHERE id = ?`, id)
	err := row.Scan(&rep.ID, &rep.Type, &rep.GuildID, &rep.ExecutorID, &rep.VictimID, &rep.Msg, &rep.AttachmentURL, &rep.Timeout)
	if err == sql.ErrNoRows {
		return nil, database.ErrDatabaseNotFound
	}

	return rep, err
}

func (m *MysqlMiddleware) GetReportsGuild(guildID string, offset, limit int) ([]*models.Report, error) {
	if limit == 0 {
		limit = 1000
	}

	rows, err := m.Db.Query(`
		SELECT id, type, guildID, executorID, victimID, msg, attachment, timeout
		FROM reports WHERE guildID = ?
		ORDER BY id DESC
		LIMIT ?, ?
	`, guildID, offset, limit)
	var results []*models.Report
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		rep := new(models.Report)
		err := rows.Scan(&rep.ID, &rep.Type, &rep.GuildID, &rep.ExecutorID,
			&rep.VictimID, &rep.Msg, &rep.AttachmentURL, &rep.Timeout)
		if err != nil {
			return nil, err
		}
		results = append(results, rep)
	}
	return results, nil
}

func (m *MysqlMiddleware) GetReportsFiltered(guildID, memberID string, repType, offset, limit int) ([]*models.Report, error) {
	args := []interface{}{}
	query := `SELECT id, type, guildID, executorID, victimID, msg, attachment, timeout FROM reports WHERE true`
	if guildID != "" {
		query += " AND guildID = ?"
		args = append(args, guildID)
	}
	if memberID != "" {
		query += " AND victimID = ?"
		args = append(args, memberID)
	}
	if repType > -1 {
		query += " AND type = ?"
		args = append(args, repType)
	}
	query += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := m.Db.Query(query, args...)
	var results []*models.Report
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		rep := new(models.Report)
		err := rows.Scan(&rep.ID, &rep.Type, &rep.GuildID, &rep.ExecutorID,
			&rep.VictimID, &rep.Msg, &rep.AttachmentURL, &rep.Timeout)
		if err != nil {
			return nil, err
		}
		results = append(results, rep)
	}
	return results, nil
}

func (m *MysqlMiddleware) GetReportsGuildCount(guildID string) (count int, err error) {
	err = m.Db.QueryRow("SELECT COUNT(id) FROM reports WHERE guildID = ?", guildID).Scan(&count)
	return
}

func (m *MysqlMiddleware) GetReportsFilteredCount(guildID, memberID string, repType int) (count int, err error) {
	if !stringutil.IsInteger(guildID) {
		err = fmt.Errorf("invalid argument type")
		return
	}

	query := fmt.Sprintf(`SELECT COUNT(id) FROM reports WHERE guildID = "%s"`, guildID)
	if memberID != "" {
		query += fmt.Sprintf(` AND victimID = "%s"`, memberID)
	}
	if repType != -1 {
		query += fmt.Sprintf(` AND type = %d`, repType)
	}

	err = m.Db.QueryRow(query).Scan(&count)
	return
}

func (m *MysqlMiddleware) GetExpiredReports() (results []*models.Report, err error) {
	rows, err := m.Db.Query(`
		SELECT id, type, guildID, executorID, victimID, msg, attachment, timeout
		FROM reports
		WHERE timeout <= CURRENT_TIMESTAMP`)
	if err != nil {
		if err == sql.ErrNoRows {
			err = nil
		}
		return
	}

	results = make([]*models.Report, 0)
	for rows.Next() {
		rep := new(models.Report)
		err := rows.Scan(&rep.ID, &rep.Type, &rep.GuildID, &rep.ExecutorID,
			&rep.VictimID, &rep.Msg, &rep.AttachmentURL, &rep.Timeout)
		if err != nil {
			return nil, err
		}
		results = append(results, rep)
	}

	return
}

func (m *MysqlMiddleware) ExpireReports(ids ...string) (err error) {
	tx, err := m.Db.Begin()
	if err != nil {
		return
	}
	for _, id := range ids {
		_, err = m.Db.Exec(`
			UPDATE reports SET timeout = NULL
			WHERE id = ?`, id)
		if err != nil {
			return
		}
	}
	err = tx.Commit()
	return
}

func (m *MysqlMiddleware) GetVotes() (map[string]*vote.Vote, error) {
	rows, err := m.Db.Query("SELECT id, data FROM votes")
	results := make(map[string]*vote.Vote)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var voteID, rawData string
		err := rows.Scan(&voteID, &rawData)
		if err != nil {
			logrus.WithError(err).Error("An error occured reading vote from database")
			continue
		}
		vote, err := vote.Unmarshal(rawData)
		if err != nil {
			m.DeleteVote(rawData)
		} else {
			results[vote.ID] = vote
		}
	}
	return results, err
}

func (m *MysqlMiddleware) AddUpdateVote(vote *vote.Vote) error {
	rawData, err := vote.Marshal()
	if err != nil {
		return err
	}

	_, err = m.Db.Exec(
		"INSERT INTO votes (id, data) VALUES (?, ?) "+
			"ON DUPLICATE KEY UPDATE data = ?", vote.ID, rawData, rawData)

	return err
}

func (m *MysqlMiddleware) DeleteVote(voteID string) error {
	_, err := m.Db.Exec("DELETE FROM votes WHERE id = ?", voteID)
	return err
}

func (m *MysqlMiddleware) GetTwitchNotify(twitchUserID, guildID string) (*twitchnotify.DBEntry, error) {
	t := &twitchnotify.DBEntry{
		TwitchUserID: twitchUserID,
		GuildID:      guildID,
	}
	err := m.Db.QueryRow("SELECT channelID FROM twitchnotify WHERE twitchUserID = ? AND guildID = ?",
		twitchUserID, guildID).Scan(&t.ChannelID)
	err = wrapNotFoundError(err)
	return t, err
}

func (m *MysqlMiddleware) SetTwitchNotify(twitchNotify *twitchnotify.DBEntry) error {
	res, err := m.Db.Exec("UPDATE twitchnotify SET channelID = ? WHERE twitchUserID = ? AND guildID = ?",
		twitchNotify.ChannelID, twitchNotify.TwitchUserID, twitchNotify.GuildID)
	if err != nil {
		return err
	}
	ar, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if ar == 0 {
		_, err = m.Db.Exec("INSERT INTO twitchnotify (twitchUserID, guildID, channelID) VALUES (?, ?, ?)",
			twitchNotify.TwitchUserID, twitchNotify.GuildID, twitchNotify.ChannelID)
	}
	return err
}

func (m *MysqlMiddleware) DeleteTwitchNotify(twitchUserID, guildID string) error {
	_, err := m.Db.Exec("DELETE FROM twitchnotify WHERE twitchUserID = ? AND guildID = ?", twitchUserID, guildID)
	return err
}

func (m *MysqlMiddleware) GetAllTwitchNotifies(twitchUserID string) ([]*twitchnotify.DBEntry, error) {
	query := "SELECT twitchUserID, guildID, channelID FROM twitchnotify"
	if twitchUserID != "" {
		query += " WHERE twitchUserID = " + twitchUserID
	}
	rows, err := m.Db.Query(query)
	results := make([]*twitchnotify.DBEntry, 0)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		t := new(twitchnotify.DBEntry)
		err = rows.Scan(&t.TwitchUserID, &t.GuildID, &t.ChannelID)
		if err == nil {
			results = append(results, t)
		}
	}
	return results, nil
}

func (m *MysqlMiddleware) AddBackup(guildID, fileID string) error {
	timestamp := time.Now().Unix()
	_, err := m.Db.Exec("INSERT INTO backups (guildID, timestamp, fileID) VALUES (?, ?, ?)", guildID, timestamp, fileID)
	return err
}

func (m *MysqlMiddleware) DeleteBackup(guildID, fileID string) error {
	_, err := m.Db.Exec("DELETE FROM backups WHERE guildID = ? AND fileID = ?", guildID, fileID)
	return err
}

func (m *MysqlMiddleware) GetGuildInviteBlock(guildID string) (string, error) {
	return m.getGuildSetting(guildID, "inviteBlock")
}

func (m *MysqlMiddleware) SetGuildInviteBlock(guildID string, data string) error {
	return m.setGuildSetting(guildID, "inviteBlock", data)
}

func (m *MysqlMiddleware) GetGuildJoinMsg(guildID string) (string, string, error) {
	data, err := m.getGuildSetting(guildID, "joinMsg")
	if err != nil {
		return "", "", err
	}
	if data == "" {
		return "", "", nil
	}

	i := strings.Index(data, "|")
	if i < 0 || len(data) < i+1 {
		return "", "", nil
	}

	return data[:i], data[i+1:], nil
}

func (m *MysqlMiddleware) SetGuildJoinMsg(guildID string, msg string, channelID string) error {
	return m.setGuildSetting(guildID, "joinMsg", fmt.Sprintf("%s|%s", msg, channelID))
}

func (m *MysqlMiddleware) GetGuildLeaveMsg(guildID string) (string, string, error) {
	data, err := m.getGuildSetting(guildID, "leaveMsg")
	if err != nil {
		return "", "", err
	}
	if data == "" {
		return "", "", nil
	}

	i := strings.Index(data, "|")
	if i < 0 || len(data) < i+1 {
		return "", "", nil
	}

	return data[:i], data[i+1:], nil
}

func (m *MysqlMiddleware) SetGuildLeaveMsg(guildID string, channelID string, msg string) error {
	return m.setGuildSetting(guildID, "leaveMsg", fmt.Sprintf("%s|%s", channelID, msg))
}

func (m *MysqlMiddleware) GetBackups(guildID string) ([]*backupmodels.Entry, error) {
	rows, err := m.Db.Query("SELECT guildID, timestamp, fileID FROM backups WHERE guildID = ?", guildID)
	if err == sql.ErrNoRows {
		return nil, database.ErrDatabaseNotFound
	}
	if err != nil {
		return nil, err
	}

	backups := make([]*backupmodels.Entry, 0)
	for rows.Next() {
		be := new(backupmodels.Entry)
		var timeStampUnix int64
		err = rows.Scan(&be.GuildID, &timeStampUnix, &be.FileID)
		if err != nil {
			return nil, err
		}
		be.Timestamp = time.Unix(timeStampUnix, 0)
		backups = append(backups, be)
	}

	return backups, nil
}

func (m *MysqlMiddleware) GetGuilds() ([]string, error) {
	rows, err := m.Db.Query("SELECT guildID FROM guilds WHERE backup = '1'")
	if err == sql.ErrNoRows {
		return nil, database.ErrDatabaseNotFound
	}
	if err != nil {
		return nil, err
	}

	guilds := make([]string, 0)
	for rows.Next() {
		var s string
		err = rows.Scan(&s)
		if err != nil {
			return nil, err
		}
		guilds = append(guilds, s)
	}

	return guilds, err
}

func (m *MysqlMiddleware) AddTag(tag *tag.Tag) error {
	_, err := m.Db.Exec("INSERT INTO tags (id, ident, creatorID, guildID, content, created, lastEdit) VALUES "+
		"(?, ?, ?, ?, ?, ?, ?)", tag.ID, tag.Ident, tag.CreatorID, tag.GuildID, tag.Content, tag.Created.Unix(), tag.LastEdit.Unix())
	return err
}

func (m *MysqlMiddleware) EditTag(tag *tag.Tag) error {
	_, err := m.Db.Exec("UPDATE tags SET "+
		"ident = ?, creatorID = ?, guildID = ?, content = ?, created = ?, lastEdit = ? "+
		"WHERE id = ?", tag.Ident, tag.CreatorID, tag.GuildID, tag.Content, tag.Created.Unix(), tag.LastEdit.Unix(), tag.ID)
	if err == sql.ErrNoRows {
		return database.ErrDatabaseNotFound
	}
	return err
}

func (m *MysqlMiddleware) GetTagByID(id snowflake.ID) (*tag.Tag, error) {
	tag := new(tag.Tag)
	var timestampCreated int64
	var timestampLastEdit int64

	row := m.Db.QueryRow("SELECT id, ident, creatorID, guildID, content, created, lastEdit FROM tags "+
		"WHERE id = ?", id)

	err := row.Scan(&tag.ID, &tag.Ident, &tag.CreatorID, &tag.GuildID,
		&tag.Content, &timestampCreated, &timestampLastEdit)
	if err == sql.ErrNoRows {
		return nil, database.ErrDatabaseNotFound
	}
	if err != nil {
		return nil, err
	}

	tag.Created = time.Unix(timestampCreated, 0)
	tag.LastEdit = time.Unix(timestampLastEdit, 0)

	return tag, nil
}

func (m *MysqlMiddleware) GetTagByIdent(ident string, guildID string) (*tag.Tag, error) {
	tag := new(tag.Tag)
	var timestampCreated int64
	var timestampLastEdit int64

	row := m.Db.QueryRow("SELECT id, ident, creatorID, guildID, content, created, lastEdit FROM tags "+
		"WHERE ident = ? AND guildID = ?", ident, guildID)

	err := row.Scan(&tag.ID, &tag.Ident, &tag.CreatorID, &tag.GuildID,
		&tag.Content, &timestampCreated, &timestampLastEdit)
	if err == sql.ErrNoRows {
		return nil, database.ErrDatabaseNotFound
	}
	if err != nil {
		return nil, err
	}

	tag.Created = time.Unix(timestampCreated, 0)
	tag.LastEdit = time.Unix(timestampLastEdit, 0)

	return tag, nil
}

func (m *MysqlMiddleware) GetGuildTags(guildID string) ([]*tag.Tag, error) {
	rows, err := m.Db.Query("SELECT id, ident, creatorID, guildID, content, created, lastEdit FROM tags "+
		"WHERE guildID = ?", guildID)
	if err == sql.ErrNoRows {
		return nil, database.ErrDatabaseNotFound
	}
	if err != nil {
		return nil, err
	}

	tags := make([]*tag.Tag, 0)
	var timestampCreated int64
	var timestampLastEdit int64
	for rows.Next() {
		tag := new(tag.Tag)
		err = rows.Scan(&tag.ID, &tag.Ident, &tag.CreatorID, &tag.GuildID,
			&tag.Content, &timestampCreated, &timestampLastEdit)
		if err != nil {
			return nil, err
		}
		tag.Created = time.Unix(timestampCreated, 0)
		tag.LastEdit = time.Unix(timestampLastEdit, 0)
		tags = append(tags, tag)
	}

	return tags, nil
}

func (m *MysqlMiddleware) DeleteTag(id snowflake.ID) error {
	_, err := m.Db.Exec("DELETE FROM tags WHERE id = ?", id)
	if err == sql.ErrNoRows {
		return database.ErrDatabaseNotFound
	}
	return err
}

func (m *MysqlMiddleware) SetAPIToken(token *models.APITokenEntry) (err error) {
	res, err := m.Db.Exec(
		"UPDATE apitokens SET "+
			"salt = ?, created = ?, expires = ?, lastAccess = ?, hits = ? "+
			"WHERE userID = ?",
		token.Salt, token.Created, token.Expires, token.LastAccess, token.Hits, token.UserID)
	if err != nil {
		return
	}

	ar, err := res.RowsAffected()
	if err != nil {
		return
	}
	if ar == 0 {
		_, err = m.Db.Exec(
			"INSERT INTO apitokens "+
				"(userID, salt, created, expires, lastAccess, hits) "+
				"VALUES (?, ?, ?, ?, ?, ?)",
			token.UserID, token.Salt, token.Created, token.Expires, token.LastAccess, token.Hits)
	}
	return
}

func (m *MysqlMiddleware) GetAPIToken(userID string) (t *models.APITokenEntry, err error) {
	t = new(models.APITokenEntry)
	err = m.Db.QueryRow(
		"SELECT userID, salt, created, expires, lastAccess, hits "+
			"FROM apitokens WHERE userID = ?", userID).
		Scan(&t.UserID, &t.Salt, &t.Created, &t.Expires, &t.LastAccess, &t.Hits)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) DeleteAPIToken(userID string) error {
	_, err := m.Db.Exec("DELETE FROM apitokens WHERE userID = ?", userID)
	if err == sql.ErrNoRows {
		return database.ErrDatabaseNotFound
	}
	return err
}

func (m *MysqlMiddleware) GetKarma(userID, guildID string) (i int, err error) {
	err = m.Db.QueryRow("SELECT value FROM karma WHERE userID = ? AND guildID = ?",
		userID, guildID).Scan(&i)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) GetKarmaSum(userID string) (i int, err error) {
	err = m.Db.QueryRow("SELECT COALESCE(SUM(value), 0) FROM karma WHERE userID = ?",
		userID).Scan(&i)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) GetKarmaGuild(guildID string, limit int) ([]*models.GuildKarma, error) {
	if limit < 1 {
		limit = 1000
	}

	res := make([]*models.GuildKarma, limit)

	rows, err := m.Db.Query(
		`SELECT userID, value FROM karma WHERE guildID = ?
		ORDER BY value DESC
		LIMIT ?`,
		guildID, limit)
	if err == sql.ErrNoRows {
		return nil, database.ErrDatabaseNotFound
	} else if err != nil {
		return nil, err
	}

	i := 0
	for rows.Next() {
		v := new(models.GuildKarma)
		v.GuildID = guildID
		if err = rows.Scan(&v.UserID, &v.Value); err != nil {
			return nil, err
		}
		res[i] = v
		i++
	}

	return res[:i], nil
}

func (m *MysqlMiddleware) SetKarma(userID, guildID string, val int) (err error) {
	res, err := m.Db.Exec("UPDATE karma SET value = ? WHERE userID = ? AND guildID = ?",
		val, userID, guildID)
	if err != nil {
		return
	}

	ar, err := res.RowsAffected()
	if err != nil {
		return
	}
	if ar == 0 {
		_, err = m.Db.Exec("INSERT INTO karma (userID, guildID, value) VALUES (?, ?, ?)",
			userID, guildID, val)
	}
	return
}

func (m *MysqlMiddleware) UpdateKarma(userID, guildID string, diff int) (err error) {
	res, err := m.Db.Exec("UPDATE karma SET value = value + ? WHERE userID = ? AND guildID = ?",
		diff, userID, guildID)
	if err != nil {
		return
	}

	ar, err := res.RowsAffected()
	if err != nil {
		return
	}
	if ar == 0 {
		_, err = m.Db.Exec("INSERT INTO karma (userID, guildID, value) VALUES (?, ?, ?)",
			userID, guildID, diff)
	}

	return
}

func (m *MysqlMiddleware) SetKarmaState(guildID string, state bool) (err error) {
	_, err = m.Db.Exec(
		"INSERT INTO karmaSettings (guildID, state) "+
			"VALUES (?, ?) "+
			"ON DUPLICATE KEY UPDATE state = ?",
		guildID, state, state)

	return
}

func (m *MysqlMiddleware) GetKarmaState(guildID string) (state bool, err error) {
	err = m.Db.QueryRow("SELECT state FROM karmaSettings WHERE guildID = ?",
		guildID).Scan(&state)
	err = wrapNotFoundError(err)

	return
}

func (m *MysqlMiddleware) SetKarmaEmotes(guildID, emotesInc, emotesDec string) (err error) {
	_, err = m.Db.Exec(
		"INSERT INTO karmaSettings (guildID, emotesInc, emotesDec) "+
			"VALUES (?, ?, ?) "+
			"ON DUPLICATE KEY UPDATE emotesInc = ?, emotesDec = ?",
		guildID, emotesInc, emotesDec, emotesInc, emotesDec)

	return
}

func (m *MysqlMiddleware) GetKarmaEmotes(guildID string) (emotesInc, emotesDec string, err error) {
	err = m.Db.QueryRow("SELECT emotesInc, emotesDec FROM karmaSettings WHERE guildID = ?",
		guildID).Scan(&emotesInc, &emotesDec)
	err = wrapNotFoundError(err)

	return
}

func (m *MysqlMiddleware) SetKarmaTokens(guildID string, tokens int) (err error) {
	_, err = m.Db.Exec(
		"INSERT INTO karmaSettings (guildID, tokens) "+
			"VALUES (?, ?) "+
			"ON DUPLICATE KEY UPDATE tokens = ?",
		guildID, tokens, tokens)

	return
}

func (m *MysqlMiddleware) GetKarmaTokens(guildID string) (tokens int, err error) {
	err = m.Db.QueryRow("SELECT tokens FROM karmaSettings WHERE guildID = ?",
		guildID).Scan(&tokens)
	err = wrapNotFoundError(err)

	return
}

func (m *MysqlMiddleware) SetKarmaPenalty(guildID string, state bool) (err error) {
	_, err = m.Db.Exec(
		"INSERT INTO karmaSettings (guildID, penalty) "+
			"VALUES (?, ?) "+
			"ON DUPLICATE KEY UPDATE penalty = ?",
		guildID, state, state)

	return
}

func (m *MysqlMiddleware) GetKarmaPenalty(guildID string) (state bool, err error) {
	err = m.Db.QueryRow("SELECT penalty FROM karmaSettings WHERE guildID = ?",
		guildID).Scan(&state)
	err = wrapNotFoundError(err)

	return
}

func (m *MysqlMiddleware) GetKarmaBlockList(guildID string) (list []string, err error) {
	row, err := m.Db.Query("SELECT userID FROM karmaBlocklist WHERE guildID = ?", guildID)
	err = wrapNotFoundError(err)
	if err != nil {
		return
	}

	list = make([]string, 0)
	var id string
	for row.Next() {
		if err = row.Scan(&id); err != nil {
			return
		}
		list = append(list, id)
	}

	return
}

func (m *MysqlMiddleware) IsKarmaBlockListed(guildID, userID string) (ok bool, err error) {
	err = m.Db.QueryRow("SELECT 1 FROM karmaBlocklist WHERE guildID = ? AND userID = ?",
		guildID, userID).Scan(&ok)
	if err != nil && err != sql.ErrNoRows {
		return
	}

	err = nil

	return
}

func (m *MysqlMiddleware) AddKarmaBlockList(guildID, userID string) (err error) {
	_, err = m.Db.Query("INSERT INTO karmaBlocklist (guildID, userID) VALUES (?, ?)",
		guildID, userID)
	return
}

func (m *MysqlMiddleware) RemoveKarmaBlockList(guildID, userID string) (err error) {
	_, err = m.Db.Query("DELETE FROM karmaBlocklist WHERE guildID = ? AND userID = ?",
		guildID, userID)
	return
}

func (m *MysqlMiddleware) SetLockChan(chanID, guildID, executorID, permissions string) error {
	_, err := m.Db.Exec("INSERT INTO chanlock (chanID, guildID, executorID, permissions) VALUES (?, ?, ?, ?)",
		chanID, guildID, executorID, permissions)
	return err
}

func (m *MysqlMiddleware) GetLockChan(chanID string) (guildID, executorID, permissions string, err error) {
	err = m.Db.QueryRow("SELECT guildID, executorID, permissions FROM chanlock WHERE chanID = ?", chanID).
		Scan(&guildID, &executorID, &permissions)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) GetLockChannels(guildID string) (chanIDs []string, err error) {
	chanIDs = make([]string, 0)
	rows, err := m.Db.Query("SELECT chanID FROM chanlock WHERE guildID = ?", guildID)
	err = wrapNotFoundError(err)
	if err != nil {
		return
	}

	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return
		}
		chanIDs = append(chanIDs, id)
	}

	return
}

func (m *MysqlMiddleware) DeleteLockChan(chanID string) error {
	_, err := m.Db.Exec("DELETE FROM chanlock WHERE chanID = ?",
		chanID)
	err = wrapNotFoundError(err)
	return err
}

func (m *MysqlMiddleware) SetAntiraidState(guildID string, state bool) (err error) {
	_, err = m.Db.Exec(
		"INSERT INTO antiraidSettings (guildID, state) "+
			"VALUES (?, ?) "+
			"ON DUPLICATE KEY UPDATE state = ?",
		guildID, state, state)

	return
}

func (m *MysqlMiddleware) GetAntiraidState(guildID string) (state bool, err error) {
	err = m.Db.QueryRow("SELECT state FROM antiraidSettings WHERE guildID = ?",
		guildID).Scan(&state)
	err = wrapNotFoundError(err)

	return
}

func (m *MysqlMiddleware) SetAntiraidRegeneration(guildID string, limit int) (err error) {
	_, err = m.Db.Exec(
		"INSERT INTO antiraidSettings (guildID, `limit`) "+
			"VALUES (?, ?) "+
			"ON DUPLICATE KEY UPDATE `limit` = ?",
		guildID, limit, limit)

	return
}

func (m *MysqlMiddleware) GetAntiraidRegeneration(guildID string) (limit int, err error) {
	err = m.Db.QueryRow("SELECT `limit` FROM antiraidSettings WHERE guildID = ?",
		guildID).Scan(&limit)
	err = wrapNotFoundError(err)

	return
}

func (m *MysqlMiddleware) SetAntiraidBurst(guildID string, burst int) (err error) {
	_, err = m.Db.Exec(
		"INSERT INTO antiraidSettings (guildID, burst) "+
			"VALUES (?, ?) "+
			"ON DUPLICATE KEY UPDATE burst = ?",
		guildID, burst, burst)

	return
}

func (m *MysqlMiddleware) GetAntiraidBurst(guildID string) (burst int, err error) {
	err = m.Db.QueryRow("SELECT burst FROM antiraidSettings WHERE guildID = ?",
		guildID).Scan(&burst)
	err = wrapNotFoundError(err)

	return
}

func (m *MysqlMiddleware) AddToAntiraidJoinList(guildID, userID, userTag string, accountCreated time.Time) (err error) {
	_, err = m.Db.Exec("INSERT IGNORE INTO antiraidJoinlog (userID, guildID, tag, accountCreated) "+
		"VALUES (?, ?, ?, ?)", userID, guildID, userTag, accountCreated)
	return
}

func (m *MysqlMiddleware) SetAntiraidVerification(guildID string, state bool) (err error) {
	_, err = m.Db.Exec(
		"INSERT INTO antiraidSettings (guildID, verification) "+
			"VALUES (?, ?) "+
			"ON DUPLICATE KEY UPDATE verification = ?",
		guildID, state, state)

	return
}

func (m *MysqlMiddleware) GetAntiraidVerification(guildID string) (state bool, err error) {
	err = m.Db.QueryRow("SELECT verification FROM antiraidSettings WHERE guildID = ?",
		guildID).Scan(&state)
	err = wrapNotFoundError(err)

	return
}

func (m *MysqlMiddleware) GetAntiraidJoinList(guildID string) (res []*models.JoinLogEntry, err error) {
	query := "SELECT `userID`, `tag`, `accountCreated`, `timestamp`, `guildID` FROM antiraidJoinlog"
	var args []interface{}

	if guildID != "" {
		query += " WHERE guildID = ?"
		args = []interface{}{guildID}
	}

	rows, err := m.Db.Query(query, args...)
	if err != nil {
		return
	}

	for rows.Next() {
		entry := &models.JoinLogEntry{GuildID: guildID}
		if err = rows.Scan(&entry.UserID, &entry.Tag, &entry.Created, &entry.Timestamp, &entry.GuildID); err != nil {
			return
		}
		res = append(res, entry)
	}

	return
}

func (m *MysqlMiddleware) FlushAntiraidJoinList(guildID string) (err error) {
	_, err = m.Db.Exec("DELETE FROM antiraidJoinlog WHERE guildID = ?", guildID)
	err = wrapNotFoundError(err)

	return
}

func (m *MysqlMiddleware) RemoveAntiraidJoinList(guildID, userID string) (err error) {
	_, err = m.Db.Exec(`DELETE FROM antiraidJoinlog WHERE guildID = ? AND userID = ?`, guildID, userID)
	err = wrapNotFoundError(err)

	return
}

func (m *MysqlMiddleware) GetGuildUnbanRequests(guildID string) (r []*models.UnbanRequest, err error) {
	rows, err := m.Db.Query(
		`SELECT id, userID, guildID, userTag, message, processedBy, status, processed, processedMessage
		FROM unbanRequests
		WHERE guildID = ?`, guildID)
	err = wrapNotFoundError(err)
	if err != nil {
		return
	}

	r = make([]*models.UnbanRequest, 0)
	for rows.Next() {
		req := new(models.UnbanRequest)
		if err = rows.Scan(
			&req.ID, &req.UserID, &req.GuildID, &req.UserTag, &req.Message,
			&req.ProcessedBy, &req.Status, &req.Processed, &req.ProcessedMessage,
		); err != nil {
			return
		}
		r = append(r, req)
	}

	return
}

func (m *MysqlMiddleware) GetGuildUserUnbanRequests(userID, guildID string) (r []*models.UnbanRequest, err error) {
	query := `SELECT id, userID, guildID, userTag, message, processedBy, status, processed, processedMessage
		FROM unbanRequests
		WHERE userID = ?`
	params := []interface{}{userID}

	if guildID != "" {
		query += " AND guildID = ?"
		params = append(params, guildID)
	}

	rows, err := m.Db.Query(query, params...)
	err = wrapNotFoundError(err)
	if err != nil {
		return
	}

	r = make([]*models.UnbanRequest, 0)
	for rows.Next() {
		req := new(models.UnbanRequest)
		if err = rows.Scan(
			&req.ID, &req.UserID, &req.GuildID, &req.UserTag, &req.Message,
			&req.ProcessedBy, &req.Status, &req.Processed, &req.ProcessedMessage,
		); err != nil {
			return
		}
		r = append(r, req)
	}

	return
}

func (m *MysqlMiddleware) GetUnbanRequest(id string) (r *models.UnbanRequest, err error) {
	row := m.Db.QueryRow(
		`SELECT id, userID, guildID, userTag, message, processedBy, status, processed, processedMessage
		FROM unbanRequests
		WHERE id = ?`, id)

	r = new(models.UnbanRequest)
	err = row.Scan(
		&r.ID, &r.UserID, &r.GuildID, &r.UserTag, &r.Message,
		&r.ProcessedBy, &r.Status, &r.Processed, &r.ProcessedMessage,
	)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) AddUnbanRequest(r *models.UnbanRequest) (err error) {
	_, err = m.Db.Exec(
		`INSERT INTO unbanRequests
		(id, userID, guildID, userTag, message, processedBy, status, processed, processedMessage)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.UserID, r.GuildID, r.UserTag, r.Message, r.ProcessedBy,
		r.Status, r.Processed, r.ProcessedMessage)

	return
}

func (m *MysqlMiddleware) UpdateUnbanRequest(r *models.UnbanRequest) (err error) {
	_, err = m.Db.Exec(
		`UPDATE unbanRequests
		SET processedBy = ?, status = ?, processed = ?, processedMessage = ?
		WHERE id = ?`,
		r.ProcessedBy, r.Status, r.Processed, r.ProcessedMessage,
		r.ID)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) GetUserOTAEnabled(userID string) (enabled bool, err error) {
	v, err := m.getUserSetting(userID, "enableOTA")
	enabled = v == "1"
	return
}

func (m *MysqlMiddleware) SetUserOTAEnabled(userID string, enabled bool) error {
	v := "0"
	if enabled {
		v = "1"
	}
	return m.setUserSetting(userID, "enableOTA", v)
}

func (m *MysqlMiddleware) GetUserVerified(userID string) (enabled bool, err error) {
	v, err := m.getUserSetting(userID, "verified")
	enabled = v == "1"
	return
}

func (m *MysqlMiddleware) SetUserVerified(userID string, enabled bool) error {
	v := "0"
	if enabled {
		v = "1"
	}
	return m.setUserSetting(userID, "verified", v)
}

func (m *MysqlMiddleware) GetUserStarboardOptout(userID string) (enabled bool, err error) {
	v, err := m.getUserSetting(userID, "starboardOptout")
	enabled = v == "1"
	return
}

func (m *MysqlMiddleware) SetUserStarboardOptout(userID string, enabled bool) error {
	v := "0"
	if enabled {
		v = "1"
	}
	return m.setUserSetting(userID, "starboardOptout", v)
}

func (m *MysqlMiddleware) GetGuildVoiceLogIgnores(guildID string) (res []string, err error) {
	row, err := m.Db.Query("SELECT channelID FROM voicelogBlocklist WHERE guildID = ?", guildID)
	err = wrapNotFoundError(err)
	if err != nil {
		return
	}

	res = make([]string, 0)
	var id string
	for row.Next() {
		if err = row.Scan(&id); err != nil {
			return
		}
		res = append(res, id)
	}

	return
}

func (m *MysqlMiddleware) IsGuildVoiceLogIgnored(guildID, channelID string) (ok bool, err error) {
	err = m.Db.QueryRow("SELECT 1 FROM voicelogBlocklist WHERE guildID = ? AND channelID = ?",
		guildID, channelID).Scan(&ok)
	if err != nil && err != sql.ErrNoRows {
		return
	}

	err = nil

	return
}

func (m *MysqlMiddleware) SetGuildVoiceLogIngore(guildID, channelID string) (err error) {
	if ok, err := m.IsGuildVoiceLogIgnored(guildID, channelID); err != nil {
		return err
	} else if ok {
		return nil
	}
	_, err = m.Db.Exec("INSERT INTO voicelogBlocklist (guildID, channelID) VALUES (?, ?)",
		guildID, channelID)
	return
}

func (m *MysqlMiddleware) RemoveGuildVoiceLogIgnore(guildID, channelID string) (err error) {
	_, err = m.Db.Exec("DELETE FROM voicelogBlocklist WHERE guildID = ? AND channelID = ?",
		guildID, channelID)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) SetStarboardConfig(config *models.StarboardConfig) (err error) {
	var ok bool
	m.Db.QueryRow("SELECT 1 FROM starboardConfig WHERE guildID = ?",
		config.GuildID).Scan(&ok)

	if ok {
		_, err = m.Db.Exec(
			"UPDATE starboardConfig SET "+
				"channelID = ?, threshold = ?, emojiID = ?, karmaGain = ? "+
				"WHERE guildID = ?",
			config.ChannelID, config.Threshold, config.EmojiID, config.KarmaGain, config.GuildID)
	} else {
		_, err = m.Db.Exec(
			"INSERT INTO starboardConfig "+
				"(guildID, channelID, threshold, emojiID, karmaGain) "+
				"VALUES (?, ?, ?, ?, ?)",
			config.GuildID, config.ChannelID, config.Threshold, config.EmojiID, config.KarmaGain)
	}

	return
}

func (m *MysqlMiddleware) GetStarboardConfig(guildID string) (config *models.StarboardConfig, err error) {
	config = new(models.StarboardConfig)

	err = m.Db.QueryRow("SELECT channelID, threshold, emojiID, karmaGain FROM starboardConfig WHERE guildID = ?", guildID).
		Scan(&config.ChannelID, &config.Threshold, &config.EmojiID, &config.KarmaGain)
	err = wrapNotFoundError(err)

	return
}

func (m *MysqlMiddleware) SetStarboardEntry(e *models.StarboardEntry) (err error) {
	var ok bool
	m.Db.QueryRow("SELECT 1 FROM starboardEntries WHERE messageID = ?",
		e.MessageID).Scan(&ok)

	if ok {
		_, err = m.Db.Exec(
			"UPDATE starboardEntries SET "+
				"score = ?, deleted = ?, starboardID = ? "+
				"WHERE messageID = ?",
			e.Score, e.Deleted, e.StarboardID, e.MessageID)
	} else {
		_, err = m.Db.Exec(
			"INSERT INTO starboardEntries "+
				"(messageID, starboardID, guildID, channelID, authorID, content, mediaURLs, score, deleted) "+
				"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			e.MessageID, e.StarboardID, e.GuildID, e.ChannelID, e.AuthorID, e.Content, e.MediaURLsEncoded(), e.Score, e.Deleted)
	}
	return
}

func (m *MysqlMiddleware) RemoveStarboardEntry(msgID string) (err error) {
	_, err = m.Db.Exec("DELETE FROM starboardEntries WHERE messageID = ?", msgID)
	return
}

func (m *MysqlMiddleware) GetStarboardEntries(
	guildID string,
	sortBy models.StarboardSortBy,
	limit, offset int,
) (res []*models.StarboardEntry, err error) {
	var sort string
	switch sortBy {
	case models.StarboardSortByLatest:
		sort = "ORDER BY starboardID DESC"
	case models.StarboardSortByMostRated:
		sort = "ORDER BY score DESC"
	}

	query := fmt.Sprintf("SELECT messageID, starboardID, guildID, channelID, authorID, content, mediaURLs, score, deleted "+
		"FROM starboardEntries "+
		"WHERE guildID = ? %s LIMIT %d OFFSET %d", sort, limit, offset)
	row, err := m.Db.Query(query, guildID)
	err = wrapNotFoundError(err)
	if err != nil {
		return
	}

	res = make([]*models.StarboardEntry, 0)
	for row.Next() {
		e := new(models.StarboardEntry)
		var mediaURLencoded string
		err = row.Scan(&e.MessageID, &e.StarboardID, &e.GuildID, &e.ChannelID, &e.AuthorID, &e.Content, &mediaURLencoded, &e.Score, &e.Deleted)
		if err != nil {
			return
		}
		if err = e.SetMediaURLs(mediaURLencoded); err != nil {
			return
		}
		res = append(res, e)
	}

	return
}

func (m *MysqlMiddleware) GetStarboardEntry(messageID string) (e *models.StarboardEntry, err error) {
	var mediaURLencoded string
	e = new(models.StarboardEntry)
	err = m.Db.QueryRow(
		"SELECT messageID, starboardID, guildID, channelID, authorID, content, mediaURLs, score, deleted "+
			"FROM starboardEntries "+
			"WHERE messageID = ?",
		messageID).
		Scan(&e.MessageID, &e.StarboardID, &e.GuildID, &e.ChannelID, &e.AuthorID, &e.Content, &mediaURLencoded, &e.Score, &e.Deleted)
	err = wrapNotFoundError(err)
	if err != nil {
		return
	}
	err = e.SetMediaURLs(mediaURLencoded)

	return
}

func (m *MysqlMiddleware) GetUserByRefreshToken(token string) (userID string, expires time.Time, err error) {
	err = m.Db.QueryRow("SELECT userID, expires FROM refreshTokens WHERE token = ?", token).Scan(
		&userID, &expires)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) SetUserRefreshToken(userID, token string, expires time.Time) (err error) {
	res, err := m.Db.Exec(
		"UPDATE refreshTokens SET "+
			"token = ?, expires = ? "+
			"WHERE userID = ?",
		token, expires, userID)
	if err != nil {
		return
	}

	ar, err := res.RowsAffected()
	if err != nil {
		return
	}
	if ar == 0 {
		_, err = m.Db.Exec(
			"INSERT INTO refreshTokens "+
				"(userID, token, expires) "+
				"VALUES (?, ?, ?)",
			userID, token, expires)
	}
	return
}

func (m *MysqlMiddleware) RevokeUserRefreshToken(userID string) (err error) {
	_, err = m.Db.Exec("DELETE FROM refreshTokens WHERE userID = ?", userID)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) CleanupExpiredRefreshTokens() (n int64, err error) {
	res, err := m.Db.Exec("DELETE FROM refreshTokens WHERE expires < CURRENT_TIMESTAMP()")
	if err != nil {
		return
	}

	n, err = res.RowsAffected()
	return
}

func (m *MysqlMiddleware) GetKarmaRules(guildID string) (res []*models.KarmaRule, err error) {
	rows, err := m.Db.Query("SELECT id, `trigger`, value, action, argument, checksum "+
		"FROM karmaRules WHERE guildID = ?", guildID)
	err = wrapNotFoundError(err)
	if err != nil {
		return
	}

	res = make([]*models.KarmaRule, 0)
	for rows.Next() {
		r := new(models.KarmaRule)
		r.GuildID = guildID
		if err = rows.Scan(&r.ID, &r.Trigger, &r.Value, &r.Action, &r.Argument, &r.Checksum); err != nil {
			return
		}
		res = append(res, r)
	}

	return
}

func (m *MysqlMiddleware) CheckKarmaRule(guildID, checksum string) (ok bool, err error) {
	err = m.Db.QueryRow("SELECT 1 FROM karmaRules WHERE guildID = ? AND checksum = ?",
		guildID, checksum).Scan(&ok)
	if err != nil && err != sql.ErrNoRows {
		return
	}

	err = nil

	return
}

func (m *MysqlMiddleware) AddOrUpdateKarmaRule(rule *models.KarmaRule) (err error) {
	var exists bool
	err = m.Db.QueryRow("SELECT 1 FROM karmaRules WHERE id = ?",
		rule.ID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return
	}

	if exists {
		_, err = m.Db.Exec("UPDATE karmaRules "+
			"SET `trigger` = ?, value = ?, action = ?, argument = ?, checksum = ? "+
			"WHERE guildID = ? AND id = ?",
			rule.Trigger, rule.Value, rule.Action, rule.Argument, rule.Checksum,
			rule.GuildID, rule.ID)
	} else {
		_, err = m.Db.Exec("INSERT INTO karmaRules "+
			"(id, guildID, `trigger`, value, action, argument, checksum) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?)",
			rule.ID, rule.GuildID, rule.Trigger, rule.Value, rule.Action, rule.Argument, rule.Checksum)
	}

	return
}

func (m *MysqlMiddleware) RemoveKarmaRule(guildID string, id snowflake.ID) (err error) {
	_, err = m.Db.Exec("DELETE FROM karmaRules WHERE guildID = ? AND id = ?", guildID, id)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) GetGuildLogDisable(guildID string) (bool, error) {
	val, err := m.getGuildSetting(guildID, "guildlogDisable")
	return val == "1", err
}

func (m *MysqlMiddleware) SetGuildLogDisable(guildID string, enabled bool) error {
	var val string
	if enabled {
		val = "1"
	}
	return m.setGuildSetting(guildID, "guildlogDisable", val)
}

func (m *MysqlMiddleware) GetGuildLogEntries(
	guildID string,
	offset, limit int,
	severity models.GuildLogSeverity,
	ascending bool,
) (res []*models.GuildLogEntry, err error) {
	order := "DESC"
	if ascending {
		order = "ASC"
	}
	rows, err := m.Db.Query(
		"SELECT id, module, message, severity, `timestamp` "+
			"FROM guildlog "+
			"WHERE guildID = ? AND (? < 0 OR severity = ?)"+
			"ORDER BY `timestamp` "+order+" "+
			"LIMIT ?, ?",
		guildID, severity, severity,
		offset, limit)
	err = wrapNotFoundError(err)
	if err != nil {
		return
	}

	res = make([]*models.GuildLogEntry, 0)
	for rows.Next() {
		r := new(models.GuildLogEntry)
		r.GuildID = guildID
		if err = rows.Scan(&r.ID, &r.Module, &r.Message, &r.Severity, &r.Timestamp); err != nil {
			return
		}
		res = append(res, r)
	}

	return
}

func (m *MysqlMiddleware) GetGuildLogEntriesCount(guildID string, severity models.GuildLogSeverity) (n int, err error) {
	err = m.Db.QueryRow(
		"SELECT COUNT(id) FROM guildlog WHERE guildID = ? AND (? < 0 OR severity = ?)",
		guildID, severity, severity).Scan(&n)
	return
}

func (m *MysqlMiddleware) AddGuildLogEntry(e *models.GuildLogEntry) (err error) {
	_, err = m.Db.Exec(
		"INSERT INTO guildlog (id, guildID, module, message, severity, `timestamp`) "+
			"VALUES (?, ?, ?, ?, ?, ?)",
		e.ID, e.GuildID, e.Module, e.Message, e.Severity, e.Timestamp)
	return
}

func (m *MysqlMiddleware) DeleteLogEntry(guildID string, id snowflake.ID) (err error) {
	_, err = m.Db.Exec("DELETE FROM guildlog WHERE guildID = ? AND id = ?",
		guildID, id)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) DeleteLogEntries(guildID string) (err error) {
	_, err = m.Db.Exec("DELETE FROM guildlog WHERE guildID = ?", guildID)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) FlushGuildData(guildID string) (err error) {
	tx, err := m.Db.Begin()
	if err != nil {
		return
	}

	deleteFrom := func(table string) error {
		_, err = tx.Exec(
			fmt.Sprintf("DELETE FROM `%s` WHERE guildID = ?", table),
			guildID)
		return err
	}

	mErr := multierror.New()
	for _, table := range guildTables {
		mErr.Append(deleteFrom(table))
	}

	if mErr.Len() > 0 {
		return mErr
	}

	return tx.Commit()
}

func (m *MysqlMiddleware) GetGuildAPI(guildID string) (settings *models.GuildAPISettings, err error) {
	settings = new(models.GuildAPISettings)
	err = m.Db.QueryRow(`SELECT enabled, origins, tokenHash FROM guildapi WHERE guildID = ?`, guildID).
		Scan(&settings.Enabled, &settings.AllowedOrigins, &settings.TokenHash)
	err = wrapNotFoundError(err)
	return
}

func (m *MysqlMiddleware) SetGuildAPI(guildID string, settings *models.GuildAPISettings) (err error) {
	var ok bool
	m.Db.QueryRow("SELECT 1 FROM guildapi WHERE guildID = ?",
		guildID).Scan(&ok)

	if ok {
		_, err = m.Db.Exec(`
			UPDATE guildapi
			SET enabled = ?, origins = ?, tokenHash = ?
			WHERE guildID = ?`,
			settings.Enabled, settings.AllowedOrigins, settings.TokenHash, guildID)
	} else {
		_, err = m.Db.Exec(`
			INSERT INTO guildapi
			(guildID, enabled, origins, tokenHash)
			VALUES (?, ?, ?, ?)`,
			guildID, settings.Enabled, settings.AllowedOrigins, settings.TokenHash)
	}

	return
}

func (m *MysqlMiddleware) GetGuildVerificationRequired(guildID string) (bool, error) {
	val, err := m.getGuildSetting(guildID, "requireUserVerification")
	return val == "1", err
}

func (m *MysqlMiddleware) SetGuildVerificationRequired(guildID string, enable bool) error {
	var val string
	if enable {
		val = "1"
	}
	return m.setGuildSetting(guildID, "requireUserVerification", val)
}

func (m *MysqlMiddleware) GetVerificationQueue(guildID, userID string) (res []*models.VerificationQueueEntry, err error) {
	var args []interface{}
	query := `
		SELECT guildID, userID, timestamp
		FROM verificationQueue
		WHERE true
	`
	if guildID != "" {
		args = append(args, guildID)
		query += " AND guildID = ?"
	}
	if userID != "" {
		args = append(args, userID)
		query += " AND userID = ?"
	}

	rows, err := m.Db.Query(query, args...)
	if err != nil {
		return
	}

	for rows.Next() {
		r := new(models.VerificationQueueEntry)
		if err = rows.Scan(&r.GuildID, &r.UserID, &r.Timestamp); err != nil {
			return
		}
		res = append(res, r)
	}
	return
}

func (m *MysqlMiddleware) FlushVerificationQueue(guildID string) (err error) {
	var args []interface{}
	query := `DELETE FROM verificationQueue`
	if guildID != "" {
		args = []interface{}{guildID}
		query += " WHERE guildID = ?"
	}
	_, err = m.Db.Exec(query, args...)
	return
}

func (m *MysqlMiddleware) AddVerificationQueue(e *models.VerificationQueueEntry) (err error) {
	res, err := m.Db.Exec(`
		UPDATE verificationQueue
		SET timestamp = ?
		WHERE guildID = ? AND userID = ?
	`, e.Timestamp, e.GuildID, e.UserID)

	affected, err := res.RowsAffected()
	if err != nil && err != sql.ErrNoRows {
		return
	}

	if affected == 0 {
		_, err = m.Db.Exec(`
			INSERT INTO verificationQueue (guildID, userID, timestamp)
			VALUES (?, ?, ?)
		`, e.GuildID, e.UserID, e.Timestamp)
	}
	return
}

func (m *MysqlMiddleware) RemoveVerificationQueue(guildID, userID string) (ok bool, err error) {
	res, err := m.Db.Exec(`
		DELETE FROM verificationQueue
		WHERE guildID = ? AND userID = ?
	`, guildID, userID)
	err = wrapNotFoundError(err)
	affected, err := res.RowsAffected()
	ok = affected > 0
	return
}

func (m *MysqlMiddleware) FlushUserData(userID string) (res map[string]int, err error) {
	res = make(map[string]int)

	r, err := m.Db.Exec(`
		UPDATE reports
		SET executorID = "000000000000000000"
		WHERE executorID = ? AND victimID != ?
	`, userID, userID)
	if err != nil && err != sql.ErrNoRows {
		return
	}
	affected, err := r.RowsAffected()
	if err != nil {
		return
	}
	res["reports"] = int(affected)

	r, err = m.Db.Exec(`
		DELETE FROM karma
		WHERE userID = ?
		AND value >= 0
	`, userID)
	if err != nil && err != sql.ErrNoRows {
		return
	}
	affected, err = r.RowsAffected()
	if err != nil {
		return
	}
	res["karma"] = int(affected)

	for _, tc := range userTables {
		r, err = m.Db.Exec(fmt.Sprintf(`
			DELETE FROM %s
			WHERE %s = ?
		`, tc.Table, tc.Column), userID)
		if err != nil && err != sql.ErrNoRows {
			return
		}
		affected, err = r.RowsAffected()
		if err != nil {
			return
		}
		res[tc.Table] = int(affected)
	}

	return
}

func (m *MysqlMiddleware) GetGuildBirthdayChan(guildID string) (chanID string, err error) {
	chanID, err = m.getGuildSetting(guildID, "birthdaychanID")
	return
}

func (m *MysqlMiddleware) SetGuildBirthdayChan(guildID string, chanID string) (err error) {
	err = m.setGuildSetting(guildID, "birthdaychanID", chanID)
	return
}

func (m *MysqlMiddleware) GetBirthdays(guildID string) (bd []*models.Birthday, err error) {
	query := "SELECT guildID, userID, `date`, showYear FROM birthdays"
	var params []interface{}

	if guildID != "" {
		query += " WHERE guildID = ?"
		params = []interface{}{guildID}
	}

	rows, err := m.Db.Query(query, params...)
	if err != nil {
		err = wrapNotFoundError(err)
		return
	}

	for rows.Next() {
		b := &models.Birthday{}
		err = rows.Scan(&b.GuildID, &b.UserID, &b.Date, &b.ShowYear)
		if err != nil {
			return
		}
		bd = append(bd, b)
	}

	return
}

func (m *MysqlMiddleware) SetBirthday(bd *models.Birthday) (err error) {
	res, err := m.Db.Exec(
		"UPDATE birthdays SET `date` = ?, showYear = ? "+
			"WHERE guildID = ? AND userID = ?",
		bd.Date, bd.ShowYear, bd.GuildID, bd.UserID)
	if err != nil {
		return wrapNotFoundError(err)
	}
	ar, err := res.RowsAffected()
	if ar == 0 {
		_, err = m.Db.Exec(
			"INSERT INTO birthdays (guildID, userID, `date`, showYear) "+
				"VALUES (?, ?, ?, ?)", bd.GuildID, bd.UserID, bd.Date, bd.ShowYear)
	}
	return wrapNotFoundError(err)
}

func (m *MysqlMiddleware) DeleteBirthday(guildID, userID string) (err error) {
	_, err = m.Db.Exec("DELETE FROM birthdays WHERE guildID = ? AND userID = ?",
		guildID, userID)
	return wrapNotFoundError(err)
}

/////////// HELPER ///////////////

func wrapNotFoundError(err error) error {
	if err == sql.ErrNoRows {
		err = database.ErrDatabaseNotFound
	}
	return err
}
