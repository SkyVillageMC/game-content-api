-- CreateTable
CREATE TABLE "ServerExtension" (
    "serverId" TEXT NOT NULL,
    "extensionId" TEXT NOT NULL,

    PRIMARY KEY ("serverId", "extensionId"),
    CONSTRAINT "ServerExtension_serverId_fkey" FOREIGN KEY ("serverId") REFERENCES "Server" ("id") ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT "ServerExtension_extensionId_fkey" FOREIGN KEY ("extensionId") REFERENCES "Extension" ("id") ON DELETE RESTRICT ON UPDATE CASCADE
);
