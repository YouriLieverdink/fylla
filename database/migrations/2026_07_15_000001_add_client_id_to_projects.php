<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        // The client mapping (ADR-0011). Nullable: an unassigned project stays
        // yours-only. Fylla-owned, set via UI (#21), never written by sync.
        Schema::table('projects', function (Blueprint $table) {
            $table->foreignId('client_id')->nullable()->constrained();
        });
    }

    public function down(): void
    {
        Schema::table('projects', function (Blueprint $table) {
            $table->dropConstrainedForeignId('client_id');
        });
    }
};
