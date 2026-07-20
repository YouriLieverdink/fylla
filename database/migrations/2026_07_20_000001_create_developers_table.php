<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    /**
     * Kendo roster (issue #55 / R2): resolves an issue's assignee_id → a
     * developer name for the Client context page. Mirrored whole from the global
     * GET /api/users by SyncKendoUsers. Only name resolution is needed today;
     * email/active/avatar are cheap and obviously useful.
     */
    public function up(): void
    {
        Schema::create('developers', function (Blueprint $table) {
            $table->id();
            $table->unsignedBigInteger('kendo_id')->unique();
            $table->string('name');
            $table->string('email')->nullable();
            $table->boolean('active')->default(true);
            $table->string('avatar_url')->nullable();
            $table->timestamps();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('developers');
    }
};
