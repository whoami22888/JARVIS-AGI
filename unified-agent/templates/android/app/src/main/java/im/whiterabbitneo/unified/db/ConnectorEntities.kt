package im.whiterabbitneo.unified.db

import androidx.room.Entity
import androidx.room.PrimaryKey

@Entity(tableName = "service_connectors")
data class ConnectorEntity(
    @PrimaryKey val id: String,
    val name: String,
    val category: String,
    val authType: String,
    val clientId: String?,
    val clientSecretCipher: String?,
    val redirectUri: String?,
    val scopesJson: String,
    val isEnabled: Boolean,
    val lastSync: Long,
    val accountEmail: String?,
)

@Entity(tableName = "connector_tokens")
data class TokenEntity(
    @PrimaryKey val connectorId: String,
    val accessTokenCipher: String,
    val refreshTokenCipher: String?,
    val tokenType: String,
    val expiresAtEpochSeconds: Long?,
    val scopesJson: String,
    val updatedAt: Long,
)

@Entity(tableName = "connector_accounts")
data class AccountEntity(
    @PrimaryKey val connectorId: String,
    val email: String?,
    val displayName: String?,
    val detailsJson: String,
    val lastSync: Long,
)