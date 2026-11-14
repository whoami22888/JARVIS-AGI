package im.whiterabbitneo.unified.db

import android.content.Context
import androidx.room.Database
import androidx.room.Room
import androidx.room.RoomDatabase

@Database(
    entities = [ConnectorEntity::class, TokenEntity::class, AccountEntity::class],
    version = 1,
    exportSchema = false,
)
abstract class ConnectorDatabase : RoomDatabase() {
    abstract fun connectorDao(): ConnectorDao

    companion object {
        fun build(context: Context): ConnectorDatabase =
            Room.databaseBuilder(context, ConnectorDatabase::class.java, "wrn_connectors.db").build()
    }
}