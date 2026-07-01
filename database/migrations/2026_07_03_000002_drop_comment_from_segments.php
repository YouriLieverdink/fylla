<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

return new class extends Migration
{
    // Per-segment comment is superseded by the notes relation (ADR-0005).
    public function up(): void
    {
        Schema::table('segments', function (Blueprint $table) {
            $table->dropColumn('comment');
        });
    }

    public function down(): void
    {
        Schema::table('segments', function (Blueprint $table) {
            $table->text('comment')->nullable();
        });
    }
};
