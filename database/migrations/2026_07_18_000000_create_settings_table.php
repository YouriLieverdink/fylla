<?php

use Illuminate\Database\Migrations\Migration;
use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Schema;

/**
 * UI-editable config (ADR-0016): a runtime override of a `config/fylla.php`
 * tuning default. The file stays the built-in default; a row exists here only
 * for a key the user has overridden. `SettingsProvider` applies these onto
 * `config('fylla.*')` on every request.
 */
return new class extends Migration
{
    public function up(): void
    {
        Schema::create('settings', function (Blueprint $table) {
            $table->id();
            $table->string('key')->unique();
            $table->json('value');
            $table->timestamps();
        });
    }

    public function down(): void
    {
        Schema::dropIfExists('settings');
    }
};
