package im.whiterabbitneo.unified.db

import androidx.room.Dao
import androidx.room.Insert
import androidx.room.OnConflictStrategy
import androidx.room.Query
import androidx.room.Update
import kotlinx.coroutines.flow.Flow

@Dao
interface ConnectorDao {
    @Query("SELECT * FROM service_connectors ORDER BY name")
    fun observeConnectors(): Flow<List<ConnectorEntity>>

    @Query("SELECT * FROM service_connectors WHERE id = :id")
    suspend fun getConnector(id: String): ConnectorEntity?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertConnector(connector: ConnectorEntity)

    @Update
    suspend fun updateConnector(connector: ConnectorEntity)

    @Query("UPDATE service_connectors SET isEnabled = :enabled, lastSync = :lastSync WHERE id = :id")
    suspend fun setEnabled(id: String, enabled: Boolean, lastSync: Long)

    @Query("UPDATE service_connectors SET clientId = :clientId, clientSecretCipher = :secretCipher, redirectUri = :redirectUri, scopesJson = :scopesJson, lastSync = :lastSync WHERE id = :id")
    suspend fun updateOAuthConfig(id: String, clientId: String?, secretCipher: String?, redirectUri: String?, scopesJson: String, lastSync: Long)

    @Query("DELETE FROM service_connectors WHERE id = :id")
    suspend fun deleteConnector(id: String)

    @Query("SELECT * FROM connector_tokens WHERE connectorId = :id")
    suspend fun getToken(id: String): TokenEntity?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertToken(token: TokenEntity)

    @Query("DELETE FROM connector_tokens WHERE connectorId = :id")
    suspend fun deleteToken(id: String)

    @Query("SELECT * FROM connector_accounts WHERE connectorId = :id")
    suspend fun getAccount(id: String): AccountEntity?

    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun upsertAccount(account: AccountEntity)

    @Query("DELETE FROM connector_accounts WHERE connectorId = :id")
    suspend fun deleteAccount(id: String)
}