<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    public function up(): void
    {
        Schema::table('issues', function (Blueprint $table) {
            // Kendo-mirror fields (ADR-0004): remaining is server-computed
            // (estimate − logged). Written by sync, never by Fylla.
            $table->integer('estimated_minutes')->nullable()->after('type');
            $table->integer('remaining_minutes')->nullable()->after('estimated_minutes');
        });
    }

    public function down(): void
    {
        Schema::table('issues', function (Blueprint $table) {
            $table->dropColumn(['estimated_minutes', 'remaining_minutes']);
        });
    }
};
