package im.whiterabbitneo.unified.ui

import im.whiterabbitneo.unified.connectors.Category
import im.whiterabbitneo.unified.connectors.ConnectorDefinition
import im.whiterabbitneo.unified.connectors.ConnectorStatus

data class ConnectorUiState(
    val definition: ConnectorDefinition,
    val status: ConnectorStatus,
    val accountEmail: String?,
    val enabled: Boolean,
    val clientId: String?,
    val redirectUri: String?,
) {
    val category: Category = definition.category
    val name: String = definition.name
    val description: String = definition.description
}