<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // Who logged the entry (ADR-0011). The mirror now holds teammates' rows
        // for managed-client projects; the personal metric filters to the user
        // via SyncedWorklog::mine().
        Schema::table('synced_worklogs', function (Blueprint $table) {
            $table->unsignedBigInteger('kendo_user_id')->nullable();
        });
    }

    public function down(): void
    {
        Schema::table('synced_worklogs', function (Blueprint $table) {
            $table->dropColumn('kendo_user_id');
        });
    }
};
