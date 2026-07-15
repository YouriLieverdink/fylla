<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('pull_requests', function (Blueprint $table) {
            // GitHub created_at, mirrored for the worklist's synthetic due date
            // (ADR-0013). Nullable: rides free in the search feed but tolerated
            // absent.
            $table->timestamp('opened_at')->nullable()->after('state');
        });
    }

    public function down(): void
    {
        Schema::table('pull_requests', function (Blueprint $table) {
            $table->dropColumn('opened_at');
        });
    }
};
